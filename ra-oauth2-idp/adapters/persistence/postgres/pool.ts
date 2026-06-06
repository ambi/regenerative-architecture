/**
 * Layer 4 — Adapter Layer (Postgres connection pool)
 *
 * node-postgres プールのラッパー。
 * 接続文字列・プールサイズ・statement timeout を環境変数から設定する。
 *
 * ADR-016 が durable state に Postgres を採用したため、本ファイルが
 * すべての Postgres adapter の接続の中心。
 *
 * 本アプリでは `pg` パッケージを `import type` で抽象化し、
 * 実 import は遅延 (dynamic import) で行うことで、Postgres を使わない
 * テスト時に `pg` 依存解決を発生させない。
 */

// pg はオプショナル依存。dynamic import で読み込む。
type PgPoolCtor = any
type PgPool = any

export interface PoolConfig {
  connectionString: string
  /** プール内最大コネクション数。slo.yaml scalability より導出。 */
  max?: number
  /** statement_timeout (ms)。slo.yaml の p99 latency × 5 を上限の目安に。 */
  statementTimeoutMs?: number
  /** idle 接続の刈り取り (ms) */
  idleTimeoutMs?: number
}

let cachedPool: PgPool | null = null

export async function getPool(config: PoolConfig): Promise<PgPool> {
  if (cachedPool) return cachedPool

  const pg = (await import('pg')) as { Pool: PgPoolCtor }
  const pool = new pg.Pool({
    connectionString: config.connectionString,
    max: config.max ?? 20,
    statement_timeout: config.statementTimeoutMs ?? 5000,
    idleTimeoutMillis: config.idleTimeoutMs ?? 30000,
    application_name: 'ra-oauth2-idp',
  })

  // 接続毎に session-level timeout を強制 (statement_timeout は接続毎)
  pool.on('connect', async (client: any) => {
    await client.query(`SET statement_timeout = ${config.statementTimeoutMs ?? 5000}`)
    await client.query(`SET idle_in_transaction_session_timeout = 30000`)
  })

  cachedPool = pool
  return pool
}

export async function closePool(): Promise<void> {
  if (cachedPool) {
    await cachedPool.end()
    cachedPool = null
  }
}

export type { PgPool }
