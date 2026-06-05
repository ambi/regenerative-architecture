/**
 * Layer 4 — Adapter Layer (Postgres OutboxEventSink)
 *
 * dual-write 問題を避けるため、ユースケースが進行中のトランザクションに
 * 「イベントを outbox テーブルに INSERT」する形でイベントを emit する。
 *
 * event-relay プロセス (infra/event-relay/main.ts) がこのテーブルを
 * tail し、Kafka に publish する。これにより:
 *   - DB へのユースケース結果と Kafka へのイベント配送が atomic に整合
 *   - Kafka が一時的に落ちても再起動後に再送できる
 *
 * トピックの割り当ては infra/event-routing.yaml の event_to_topic
 * を権威として、本コード内に定数として持つ。CI で整合性検証する。
 */

import type { EventSink } from '../../../src/ports/event-sink'
import type { TransactionContext } from '../../../src/ports/transaction'
import type { DomainEvent } from '../../../src/spec-bindings/schemas'
import type { PgPool } from './pool'

// infra/event-routing.yaml の event_to_topic と一致しなければならない。
// CI (infra/scripts/check-spec-coherence.ts) で機械検証する。
const EVENT_TO_TOPIC: Record<string, string> = {
  ClientRegistered: 'oauth2.client.lifecycle.v1',
  UserAuthenticated: 'oauth2.authentication.v1',
  AuthenticationFailed: 'oauth2.authentication.v1',
  ConsentGranted: 'oauth2.consent.v1',
  ConsentRevoked: 'oauth2.consent.v1',
  AuthorizationCodeIssued: 'oauth2.authorization-code.v1',
  AuthorizationCodeRedeemed: 'oauth2.authorization-code.v1',
  AccessTokenIssued: 'oauth2.token.v1',
  RefreshTokenIssued: 'oauth2.token.v1',
  RefreshTokenRotated: 'oauth2.token.v1',
  TokenRevoked: 'oauth2.token.v1',
  TokenIntrospected: 'oauth2.token.v1',
  RefreshTokenReuseDetected: 'oauth2.security-incident.v1',
  SigningKeyRotated: 'oauth2.key-management.v1',
  PARStored: 'oauth2.par.v1',
  DeviceAuthorizationRequested: 'oauth2.device-authorization.v1',
  DeviceAuthorizationApproved: 'oauth2.device-authorization.v1',
  DeviceAuthorizationDenied: 'oauth2.device-authorization.v1',
}

export class PostgresOutboxEventSink implements EventSink {
  constructor(private readonly pool: PgPool) {}

  async publish(event: DomainEvent, tx?: TransactionContext): Promise<void> {
    const querier: any = tx ?? this.pool
    const topic = EVENT_TO_TOPIC[event.type]
    if (!topic) {
      throw new Error(`No topic mapping for event type: ${event.type}`)
    }
    await querier.query(
      `
      INSERT INTO outbox (event_type, topic, payload)
      VALUES ($1, $2, $3::jsonb)
      `,
      [event.type, topic, JSON.stringify(event)],
    )
  }

  async publishMany(events: DomainEvent[], tx?: TransactionContext): Promise<void> {
    for (const e of events) await this.publish(e, tx)
  }
}
