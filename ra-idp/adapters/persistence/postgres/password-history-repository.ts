/**
 * Layer 4 — Adapter Layer (Postgres PasswordHistoryRepository)
 *
 * password_history テーブルは ADR-027 / infra/migrations/0004_password_history.sql
 * に従う。recent() は depth 件まで created_at DESC で返す。剪定は usecase 側で
 * 行わず、depth 外のエントリは "見えない" 状態にして残す (将来 depth を増やす
 * テナント運用での後方互換のため)。
 */

import type {
  PasswordHistoryEntry,
  PasswordHistoryRepository,
} from '../../../src/authentication/ports/password-history-repository'
import type { PgPool } from './pool'

function rowToEntry(row: any): PasswordHistoryEntry {
  return {
    encoded: row.encoded,
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
  }
}

export class PostgresPasswordHistoryRepository implements PasswordHistoryRepository {
  constructor(private readonly pool: PgPool) {}

  async recent(sub: string, depth: number): Promise<PasswordHistoryEntry[]> {
    if (depth <= 0) return []
    const { rows } = await this.pool.query(
      `SELECT encoded, created_at
       FROM password_history
       WHERE sub = $1
       ORDER BY created_at DESC, id DESC
       LIMIT $2`,
      [sub, depth],
    )
    return rows.map(rowToEntry)
  }

  async add(sub: string, encoded: string, now: Date): Promise<void> {
    await this.pool.query(
      `INSERT INTO password_history (sub, encoded, created_at) VALUES ($1, $2, $3)`,
      [sub, encoded, now.toISOString()],
    )
  }
}
