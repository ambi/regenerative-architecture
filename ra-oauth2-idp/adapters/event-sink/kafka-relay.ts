/**
 * Layer 4 — Adapter Layer (Kafka relay)
 *
 * outbox テーブルを tail し、未配送イベントを Kafka に publish する relay。
 * infra/event-relay/main.ts が本クラスを起動して常駐ループを回す想定。
 *
 * ADR-016: at-least-once 配送、トピックは infra/event-routing.yaml
 * の event_to_topic に従う (outbox テーブルの topic カラムに保存済み)。
 *
 * 設計:
 *   - SELECT ... FOR UPDATE SKIP LOCKED で worker をスケール可能に
 *   - publish 成功 → UPDATE outbox SET published_at = now()
 *   - 失敗 → attempts++, last_error 更新、次回ポーリングで再試行
 *   - partition key は infra/event-routing.yaml の topics.<name>.partition_key
 *     (sub / familyId / clientId 等) を payload から抽出
 */

import type { PgPool } from '../persistence/postgres/pool'

type KafkaCtor = any
type Producer = any

export interface KafkaRelayConfig {
  brokers: string[]
  pollIntervalMs?: number
  batchSize?: number
  clientId?: string
}

// payload のフィールドから partition key を抽出する関数。
// infra/event-routing.yaml の topics.<name>.partition_key と一致させる。
const KEY_EXTRACTOR: Record<string, (payload: any) => string | undefined> = {
  ClientRegistered: (p) => p.clientId,
  UserAuthenticated: (p) => p.sub,
  AuthenticationFailed: (p) => p.username,
  ConsentGranted: (p) => p.sub,
  ConsentRevoked: (p) => p.sub,
  AuthorizationCodeIssued: (p) => p.sub,
  AuthorizationCodeRedeemed: (p) => p.sub,
  AccessTokenIssued: (p) => p.sub,
  RefreshTokenIssued: (p) => p.familyId,
  RefreshTokenRotated: (p) => p.familyId,
  TokenRevoked: (p) => p.tokenId,
  TokenIntrospected: (p) => p.rsClientId,
  RefreshTokenReuseDetected: (p) => p.familyId,
  SigningKeyRotated: (p) => p.newKid,
  PARStored: (p) => p.clientId,
}

export class KafkaOutboxRelay {
  private producer: Producer | null = null
  private running = false
  private timer: ReturnType<typeof setTimeout> | null = null

  constructor(
    private readonly pool: PgPool,
    private readonly config: KafkaRelayConfig,
  ) {}

  async start(): Promise<void> {
    if (this.running) return
    const mod = (await import('kafkajs')) as any
    const Kafka: KafkaCtor = mod.Kafka
    const kafka = new Kafka({
      clientId: this.config.clientId ?? 'ra-oauth2-idp-event-relay',
      brokers: this.config.brokers,
    })
    this.producer = kafka.producer({ idempotent: true })
    await this.producer.connect()
    this.running = true
    this.loop()
  }

  async stop(): Promise<void> {
    this.running = false
    if (this.timer) {
      clearTimeout(this.timer)
      this.timer = null
    }
    if (this.producer) {
      await this.producer.disconnect()
      this.producer = null
    }
  }

  private loop(): void {
    if (!this.running) return
    this.tick()
      .catch((err) => {
        // eslint-disable-next-line no-console
        console.error('[kafka-relay] loop error:', err)
      })
      .finally(() => {
        this.timer = setTimeout(() => this.loop(), this.config.pollIntervalMs ?? 200)
      })
  }

  private async tick(): Promise<void> {
    if (!this.producer) return
    const batchSize = this.config.batchSize ?? 100
    const client = await this.pool.connect()
    try {
      await client.query('BEGIN')
      const { rows } = await client.query(
        `
        SELECT id, event_type, topic, payload
        FROM outbox
        WHERE published_at IS NULL
        ORDER BY id
        FOR UPDATE SKIP LOCKED
        LIMIT $1
        `,
        [batchSize],
      )
      if (rows.length === 0) {
        await client.query('COMMIT')
        return
      }

      // トピックごとにグルーピングして単一 producer.send にする
      const byTopic = new Map<string, Array<{ id: number; key?: string; value: string }>>()
      for (const r of rows) {
        const extractor = KEY_EXTRACTOR[r.event_type]
        const key = extractor ? extractor(r.payload) : undefined
        const list = byTopic.get(r.topic) ?? []
        list.push({
          id: r.id,
          key,
          value: JSON.stringify(r.payload),
        })
        byTopic.set(r.topic, list)
      }

      const publishedIds: number[] = []
      for (const [topic, messages] of byTopic) {
        try {
          await this.producer.send({
            topic,
            messages: messages.map((m) => ({ key: m.key, value: m.value })),
          })
          for (const m of messages) publishedIds.push(m.id)
        } catch (err) {
          // batch 単位で失敗したら次回再試行 (at-least-once)
          await client.query(
            `
            UPDATE outbox
            SET attempts = attempts + 1, last_error = $1
            WHERE id = ANY($2::bigint[])
            `,
            [String((err as Error).message).slice(0, 500), messages.map((m) => m.id)],
          )
        }
      }

      if (publishedIds.length > 0) {
        await client.query(
          `
          UPDATE outbox
          SET published_at = now(),
              published_to = 'kafka',
              attempts = attempts + 1
          WHERE id = ANY($1::bigint[])
          `,
          [publishedIds],
        )
      }
      await client.query('COMMIT')
    } catch (err) {
      await client.query('ROLLBACK')
      throw err
    } finally {
      client.release()
    }
  }
}
