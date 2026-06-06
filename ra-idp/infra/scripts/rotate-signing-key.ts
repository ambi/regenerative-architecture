/**
 * 署名鍵ローテーションの運用エントリポイント (ADR-009)。
 *
 * 90 日ごとの定期回転、または鍵漏洩時の緊急回転に使う。
 * Kubernetes CronJob / GitHub Actions scheduled workflow から呼ぶ想定。
 *
 * 使い方:
 *   PERSISTENCE=postgres DATABASE_URL=postgres://... \
 *   EVENT_SINK=outbox \
 *     bun run rotate:key
 *
 *   # in-memory (デモ/検証のみ。プロセス内でしか効かない):
 *   bun run rotate:key
 *
 * 終了コード 0 = 回転成功 (SigningKeyRotated を発行)。
 */

import type { KeyStore } from '../../src/oauth2/ports/key-store'
import type { EventSink } from '../../src/shared/ports/event-sink'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import { rotateSigningKeyUseCase } from '../../src/oauth2/usecases/rotate-signing-key'
import { ConsoleEventSink } from '../../adapters/event-sink/console'

async function main(): Promise<void> {
  const persistence = (process.env.PERSISTENCE ?? 'memory') as 'memory' | 'postgres'
  const eventSinkMode = (process.env.EVENT_SINK ?? 'console') as 'console' | 'outbox'
  const alg = (process.env.SIGNING_ALG ?? 'PS256') as 'PS256' | 'ES256'

  let keyStore: KeyStore
  let eventSink: EventSink
  let cleanup: () => Promise<void> = async () => {}

  if (persistence === 'postgres') {
    const dbUrl = process.env.DATABASE_URL
    if (!dbUrl) throw new Error('PERSISTENCE=postgres requires DATABASE_URL')
    const { getPool, closePool } = await import('../../adapters/persistence/postgres/pool')
    const pool = await getPool({ connectionString: dbUrl })
    const { PostgresKeyStore } = await import('../../adapters/persistence/postgres/key-store')
    keyStore = await PostgresKeyStore.create(pool, alg)
    eventSink =
      eventSinkMode === 'outbox'
        ? new (
            await import('../../adapters/persistence/postgres/outbox-event-sink')
          ).PostgresOutboxEventSink(pool)
        : new ConsoleEventSink()
    cleanup = async () => {
      await closePool()
    }
  } else {
    const { InMemoryKeyStore } = await import('../../adapters/crypto/in-memory-key-store')
    keyStore = await InMemoryKeyStore.create(alg)
    eventSink = new ConsoleEventSink()
  }

  const pending: Promise<void>[] = []
  const emit = (e: DomainEvent): void => {
    pending.push(eventSink.publish(e))
  }

  const result = await rotateSigningKeyUseCase({ keyStore }, emit)
  await Promise.all(pending)
  if (eventSink.close) await eventSink.close()
  await cleanup()

  // eslint-disable-next-line no-console
  console.log(
    `[rotate-signing-key] rotated: newKid=${result.newKid}` +
      (result.previousKid ? ` previousKid=${result.previousKid}` : ' (initial key)'),
  )
}

main()
  .then(() => process.exit(0))
  .catch((e) => {
    console.error(e)
    process.exit(1)
  })
