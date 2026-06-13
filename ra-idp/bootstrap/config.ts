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
  /**
   * テナント prefix が付かない base issuer (e.g. `https://idp.example.com`)。
   * 実 issuer は通常 `{issuer}/realms/{tenant_id}` となる (ADR-033 §3)。
   * `legacyBareIssuer=true` のときは default テナントの bare 経路でこの base が
   * そのまま `iss` claim になる (1 リリース限定の暫定措置)。
   */
  issuer: string
  persistenceMode: PersistenceMode
  eventSinkMode: EventSinkMode
  observabilityMode: ObservabilityMode
  legacyBareIssuer: boolean
}

export function loadConfig(): RuntimeConfig {
  const port = Number(process.env.PORT ?? 3000)
  const issuer = process.env.ISSUER ?? `http://localhost:${port}`
  const persistenceMode = (process.env.PERSISTENCE ?? 'memory') as PersistenceMode
  const eventSinkMode = (process.env.EVENT_SINK ?? 'console') as EventSinkMode
  const observabilityMode = (process.env.OBSERVABILITY ?? 'noop') as ObservabilityMode
  const legacyBareIssuer = process.env.LEGACY_BARE_ISSUER === 'true'
  return { port, issuer, persistenceMode, eventSinkMode, observabilityMode, legacyBareIssuer }
}
