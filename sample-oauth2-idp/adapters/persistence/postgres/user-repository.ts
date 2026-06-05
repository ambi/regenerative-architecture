/**
 * Layer 4 — Adapter Layer (Postgres UserRepository)
 *
 * GDPR / 個人情報保護要件 (spec/scl.yaml models.User の pii 注釈) は
 * infra/migrations/0001_init.sql 内の users テーブルのカラムコメントとして表現済み。
 * deleted_at 後 30 日の PII purge はバッチで実行する想定 (本サンプルでは未実装、
 * Phase 3 で運用ジョブとして cron 化する)。
 */

import { UserSchema, type User } from '../../../src/spec-bindings/schemas'
import type { UserRepository } from '../../../src/ports/user-repository'
import type { PgPool } from './pool'

function rowToUser(row: any): User {
  return UserSchema.parse({
    sub: row.sub,
    preferred_username: row.preferred_username,
    password_hash: row.password_hash,
    name: row.name ?? undefined,
    given_name: row.given_name ?? undefined,
    family_name: row.family_name ?? undefined,
    email: row.email ?? undefined,
    email_verified: row.email_verified,
    mfa_enrolled: row.mfa_enrolled,
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
    updated_at: row.updated_at instanceof Date ? row.updated_at.toISOString() : row.updated_at,
    deleted_at: row.deleted_at
      ? row.deleted_at instanceof Date
        ? row.deleted_at.toISOString()
        : row.deleted_at
      : undefined,
  })
}

export class PostgresUserRepository implements UserRepository {
  constructor(private readonly pool: PgPool) {}

  async findBySub(sub: string): Promise<User | null> {
    const { rows } = await this.pool.query(
      `SELECT * FROM users WHERE sub = $1 AND deleted_at IS NULL`,
      [sub],
    )
    return rows[0] ? rowToUser(rows[0]) : null
  }

  async findByUsername(username: string): Promise<User | null> {
    const { rows } = await this.pool.query(
      `SELECT * FROM users WHERE preferred_username = $1 AND deleted_at IS NULL`,
      [username],
    )
    return rows[0] ? rowToUser(rows[0]) : null
  }

  async save(user: User): Promise<void> {
    await this.pool.query(
      `
      INSERT INTO users (
        sub, preferred_username, password_hash,
        name, given_name, family_name, email,
        email_verified, mfa_enrolled,
        created_at, updated_at, deleted_at
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
      ON CONFLICT (sub) DO UPDATE SET
        preferred_username = EXCLUDED.preferred_username,
        password_hash      = EXCLUDED.password_hash,
        name               = EXCLUDED.name,
        given_name         = EXCLUDED.given_name,
        family_name        = EXCLUDED.family_name,
        email              = EXCLUDED.email,
        email_verified     = EXCLUDED.email_verified,
        mfa_enrolled       = EXCLUDED.mfa_enrolled,
        updated_at         = EXCLUDED.updated_at,
        deleted_at         = EXCLUDED.deleted_at
      `,
      [
        user.sub,
        user.preferred_username,
        user.password_hash,
        user.name ?? null,
        user.given_name ?? null,
        user.family_name ?? null,
        user.email ?? null,
        user.email_verified,
        user.mfa_enrolled,
        user.created_at,
        user.updated_at,
        user.deleted_at ?? null,
      ],
    )
  }
}
