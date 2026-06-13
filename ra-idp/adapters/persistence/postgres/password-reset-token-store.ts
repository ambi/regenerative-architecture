/**
 * Layer 4 — Adapter Layer (Postgres PasswordResetTokenStore)
 *
 * 0005_password_reset_tokens.sql / ADR-030 に対応する Postgres adapter。
 * consume は DELETE ... RETURNING で原子的に行う。期限切れ行は GC 用に
 * インデックスを張ってあるが、本 adapter では DELETE 時に窓を見て弾く。
 */

import type {
  PasswordResetTokenRecord,
  PasswordResetTokenStore,
} from '../../../src/authentication/ports/password-reset-token-store'
import type { PgPool } from './pool'

function rowToRecord(row: any): PasswordResetTokenRecord {
  return {
    sub: row.sub,
    token_hash: row.token_hash,
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
    expires_at: row.expires_at instanceof Date ? row.expires_at.toISOString() : row.expires_at,
  }
}

export class PostgresPasswordResetTokenStore implements PasswordResetTokenStore {
  constructor(private readonly pool: PgPool) {}

  async save(record: PasswordResetTokenRecord): Promise<void> {
    const client = await this.pool.connect()
    try {
      await client.query('BEGIN')
      await client.query(`DELETE FROM password_reset_tokens WHERE sub = $1`, [record.sub])
      await client.query(
        `INSERT INTO password_reset_tokens (token_hash, sub, created_at, expires_at)
         VALUES ($1, $2, $3, $4)`,
        [record.token_hash, record.sub, record.created_at, record.expires_at],
      )
      await client.query('COMMIT')
    } catch (e) {
      await client.query('ROLLBACK')
      throw e
    } finally {
      client.release()
    }
  }

  async consume(tokenHash: string, now: Date): Promise<PasswordResetTokenRecord | null> {
    const { rows } = await this.pool.query(
      `DELETE FROM password_reset_tokens
       WHERE token_hash = $1
       RETURNING token_hash, sub, created_at, expires_at`,
      [tokenHash],
    )
    if (rows.length === 0) return null
    const record = rowToRecord(rows[0])
    if (Date.parse(record.expires_at) <= now.getTime()) return null
    return record
  }
}
