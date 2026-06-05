/**
 * Layer 4 — Adapter Layer (Console EventSink)
 *
 * ローカル開発・サンプル用。イベントを JSON Lines として stdout に書く。
 * Phase 2 で OpenTelemetry の logger に置き換えるが、開発時の生 stdout 出力は
 * 依然として価値があるため残す。
 */

import type { EventSink } from '../../src/ports/event-sink'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export class ConsoleEventSink implements EventSink {
  constructor(private readonly opts: { collect?: boolean } = {}) {}

  private readonly log: DomainEvent[] = []

  async publish(event: DomainEvent): Promise<void> {
    // eslint-disable-next-line no-console
    console.log(JSON.stringify({ '@type': 'oauth2.domain-event', ...event }))
    if (this.opts.collect) this.log.push(event)
  }

  /** デモ用: /events エンドポイントが返す履歴。 */
  getCollected(): DomainEvent[] {
    return [...this.log]
  }
}
