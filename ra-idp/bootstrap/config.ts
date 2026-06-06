/**
 * Layer 5 — Runtime: 環境変数から起動構成を読み出す。
 *
 * すべての env アクセスは本ファイルに閉じる。下流レイヤは RuntimeConfig 経由で
 * 値を受け取り process.env を直接見ない。
 */

export type PersistenceMode = 'memory' | 'postgres'
export type EventSinkMode = 'console' | 'outbox'
export type ObservabilityMode = 'noop' | 'otel'

export interface RuntimeConfig {
  port: number
  issuer: string
  persistenceMode: PersistenceMode
  eventSinkMode: EventSinkMode
  observabilityMode: ObservabilityMode
}

export function loadConfig(): RuntimeConfig {
  const port = Number(process.env.PORT ?? 3000)
  const issuer = process.env.ISSUER ?? `http://localhost:${port}`
  const persistenceMode = (process.env.PERSISTENCE ?? 'memory') as PersistenceMode
  const eventSinkMode = (process.env.EVENT_SINK ?? 'console') as EventSinkMode
  const observabilityMode = (process.env.OBSERVABILITY ?? 'noop') as ObservabilityMode
  return { port, issuer, persistenceMode, eventSinkMode, observabilityMode }
}
