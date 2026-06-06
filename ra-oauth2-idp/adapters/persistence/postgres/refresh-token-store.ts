/**
 * Layer 4 — Adapter Layer (Postgres RefreshTokenStore)
 *
 * ADR-004 (rotation + family revoke) を Postgres で実装。
 *
 * rotate() は SELECT FOR UPDATE で parent をロックし、
 *   - すでに rotated/revoked なら null を返す (再利用検出)
 *   - そうでなければ parent を rotated にし、新トークンを INSERT
 * を 1 つの不可分操作として行う。
 *
 * revokeFamily() は family_id をキーに UPDATE。
 * トランザクション内で呼ばれる前提なら usecase 側の tx に参加する。
 */

import {
  RefreshTokenRecordSchema,
  type RefreshTokenRecord,
} from '../../../src/spec-bindings/schemas'
import type { RefreshTokenStore } from '../../../src/ports/refresh-token-store'
import type { PgPool } from './pool'

function rowToRecord(row: any): RefreshTokenRecord {
  return RefreshTokenRecordSchema.parse({
    id: row.id,
    hash: row.hash,
    family_id: row.family_id,
    parent_id: row.parent_id ?? undefined,
    client_id: row.client_id,
    sub: row.sub,
    scopes: row.scopes,
    issued_at: row.issued_at instanceof Date ? row.issued_at.toISOString() : row.issued_at,
    expires_at: row.expires_at instanceof Date ? row.expires_at.toISOString() : row.expires_at,
    absolute_expires_at:
      row.absolute_expires_at instanceof Date
        ? row.absolute_expires_at.toISOString()
        : row.absolute_expires_at,
    revoked: row.revoked,
    rotated: row.rotated,
    sender_constraint: row.sender_constraint ?? null,
  })
}

export class PostgresRefreshTokenStore implements RefreshTokenStore {
  constructor(private readonly pool: PgPool) {}

  async findByHash(hash: string): Promise<RefreshTokenRecord | null> {
    const { rows } = await this.pool.query(`SELECT * FROM refresh_tokens WHERE hash = $1`, [hash])
    return rows[0] ? rowToRecord(rows[0]) : null
  }

  async save(record: RefreshTokenRecord): Promise<void> {
    await this.pool.query(
      `
      INSERT INTO refresh_tokens (
        id, hash, family_id, parent_id, client_id, sub, scopes,
        issued_at, expires_at, absolute_expires_at,
        revoked, rotated, sender_constraint
      ) VALUES (
        $1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13::jsonb
      )
      `,
      [
        record.id,
        record.hash,
        record.family_id,
        record.parent_id ?? null,
        record.client_id,
        record.sub,
        JSON.stringify(record.scopes),
        record.issued_at,
        record.expires_at,
        record.absolute_expires_at,
        record.revoked,
        record.rotated,
        record.sender_constraint ? JSON.stringify(record.sender_constraint) : null,
      ],
    )
  }

  async rotate(
    parentId: string,
    newRecord: RefreshTokenRecord,
  ): Promise<RefreshTokenRecord | null> {
    const client = await this.pool.connect()
    try {
      await client.query('BEGIN')
      // 親トークンをロック取得
      const parentResult = await client.query(
        `SELECT * FROM refresh_tokens WHERE id = $1 FOR UPDATE`,
        [parentId],
      )
      const parent = parentResult.rows[0]
      if (!parent) {
        await client.query('ROLLBACK')
        return null
      }
      if (parent.rotated || parent.revoked) {
        // 並行 rotate またはリプレイ。usecase 側がファミリー失効を呼び出す責務。
        await client.query('ROLLBACK')
        return null
      }
      // parent を rotated にし、新トークンを挿入
      await client.query(`UPDATE refresh_tokens SET rotated = TRUE WHERE id = $1`, [parentId])
      await client.query(
        `
        INSERT INTO refresh_tokens (
          id, hash, family_id, parent_id, client_id, sub, scopes,
          issued_at, expires_at, absolute_expires_at,
          revoked, rotated, sender_constraint
        ) VALUES (
          $1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13::jsonb
        )
        `,
        [
          newRecord.id,
          newRecord.hash,
          newRecord.family_id,
          newRecord.parent_id ?? null,
          newRecord.client_id,
          newRecord.sub,
          JSON.stringify(newRecord.scopes),
          newRecord.issued_at,
          newRecord.expires_at,
          newRecord.absolute_expires_at,
          newRecord.revoked,
          newRecord.rotated,
          newRecord.sender_constraint ? JSON.stringify(newRecord.sender_constraint) : null,
        ],
      )
      await client.query('COMMIT')
      return newRecord
    } catch (err) {
      await client.query('ROLLBACK')
      throw err
    } finally {
      client.release()
    }
  }

  async revokeFamily(family_id: string): Promise<void> {
    await this.pool.query(`UPDATE refresh_tokens SET revoked = TRUE WHERE family_id = $1`, [
      family_id,
    ])
  }
}
