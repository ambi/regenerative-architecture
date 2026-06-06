/**
 * Layer 4 — Adapter Layer（HTTP: /health）
 *
 * 起動時に bootstrap が決定した runtime ラベルをそのまま返すだけの簡易ヘルス
 * エンドポイント。他のエンドポイントと同様に createXRoutes(deps) 形式で統一する。
 */

import { Hono } from 'hono'

export interface HealthInfo {
  persistence: string
  event_sink: string
  observability: string
}

export interface HealthRoutesDeps {
  issuer: string
  healthInfo: HealthInfo
}

export function createHealthRoutes(deps: HealthRoutesDeps) {
  const app = new Hono()

  app.get('/health', (c) =>
    c.json({
      status: 'ok',
      issuer: deps.issuer,
      persistence: deps.healthInfo.persistence,
      event_sink: deps.healthInfo.event_sink,
      observability: deps.healthInfo.observability,
    }),
  )

  return app
}
