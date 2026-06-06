/**
 * Layer 4 — Adapter Layer（HTTP: /events）
 *
 * memory モードの場合のみイベント履歴を返す。本番 (postgres + outbox/kafka) では
 * SIEM 側で参照する設計のためサーバー内に履歴を持たない。このエンドポイントは
 * bootstrap 側で collectedConsoleEvents が存在する場合のみマウントされる。
 */

import { Hono } from 'hono'
import type { ConsoleEventSink } from '../event-sink/console'

export interface EventsRoutesDeps {
  collectedEvents: ConsoleEventSink
}

export function createEventsRoutes(deps: EventsRoutesDeps) {
  const app = new Hono()

  app.get('/events', (c) => c.json(deps.collectedEvents.getCollected()))

  return app
}
