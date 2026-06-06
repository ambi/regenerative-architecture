/**
 * Layer 5 — Runtime: エントリポイント。
 *
 * 起動シーケンスは bootstrap/run.ts に集約。ランタイム (Node / Bun / Deno / Lambda)
 * の差し替えは本ファイルだけを変えれば済む (例: Lambda なら handler を別 export)。
 *
 * 環境変数:
 *   PORT                  3000
 *   ISSUER                http://localhost:${PORT}
 *   PERSISTENCE           memory | postgres            (default: memory)
 *   EVENT_SINK            console | outbox             (default: console)
 *   OBSERVABILITY         noop | otel                  (default: noop)
 *   DATABASE_URL          postgres://...               (PERSISTENCE=postgres or EVENT_SINK=outbox 時に必須)
 *   REDIS_URL             redis://...                  (PERSISTENCE=postgres 時に必須)
 *   OTEL_EXPORTER_OTLP_ENDPOINT  http://...:4318       (OBSERVABILITY=otel 時に推奨)
 *   DEMO_CLIENT_SECRET    任意                          (デモクライアント用)
 *   SKIP_DEMO_SEED        any                          (本番起動時の seed スキップ)
 */

import { run } from './bootstrap/run'

const { port, fetch } = await run()

export default { port, fetch }
