/**
 * Layer 5 — Runtime: EventSink ポートを emit クロージャに射影。
 *
 * createXRoutes は emit(event) という同期的なクロージャを受け取る契約。失敗は
 * 内部でログに残す責務 (fire-and-forget)。Phase 2 で構造化ログ化する。
 */

import type { EventSink } from '../src/shared/ports/event-sink'
import type { DomainEvent } from '../src/spec-bindings/schemas'

export function createEmitter(eventSink: EventSink): (event: DomainEvent) => void {
  return (event: DomainEvent) => {
    eventSink.publish(event).catch((err) => {
      // eslint-disable-next-line no-console
      console.error('[event-sink] publish failed:', err)
    })
  }
}
