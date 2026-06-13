/**
 * Layer 4 — Adapter Layer (Postgres UserRepository)
 *
 * GDPR / 個人情報保護要件 (spec/scl.yaml models.User の pii 注釈) は
 * infra/migrations/0001_init.sql 内の users テーブルのカラムコメントとして表現済み。
 * deleted_at 後 30 日の PII purge はバッチで実行する想定 (本アプリでは未実装、
 * Phase 3 で運用ジョブとして cron 化する)。
 */

import { UserSchema, type User } from '../../../src/spec-bindings/schemas'
import type { UserRepository } from '../../../src/authentication/ports/user-repository'
import type { PgPool } from './pool'

function toIso(value: unknown): string | undefined {
  if (!value) return undefined
  return value instanceof Date ? value.toISOString() : String(value)
}

function rowToUser(row: any): User {
  return UserSchema.parse({
    sub: row.sub,
    tenant_id: row.tenant_id,
    preferred_username: row.preferred_username,
    password_hash: row.password_hash,
    name: row.name ?? undefined,
    given_name: row.given_name ?? undefined,
    family_name: row.family_name ?? undefined,
    email: row.email ?? undefined,
    email_verified: row.email_verified,
    mfa_enrolled: row.mfa_enrolled,
    roles: Array.isArray(row.roles) ? row.roles : [],
    disabled_at: toIso(row.disabled_at),
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
    updated_at: row.updated_at instanceof Date ? row.updated_at.toISOString() : row.updated_at,
    deleted_at: toIso(row.deleted_at),
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

  async findByUsername(tenant_id: string, username: string): Promise<User | null> {
    const { rows } = await this.pool.query(
      `SELECT * FROM users WHERE tenant_id = $1 AND preferred_username = $2 AND deleted_at IS NULL`,
      [tenant_id, username],
    )
    return rows[0] ? rowToUser(rows[0]) : null
  }

  async findByEmail(tenant_id: string, email: string): Promise<User | null> {
    const { rows } = await this.pool.query(
      `SELECT * FROM users WHERE tenant_id = $1 AND LOWER(email) = LOWER($2) AND deleted_at IS NULL LIMIT 1`,
      [tenant_id, email],
    )
    return rows[0] ? rowToUser(rows[0]) : null
  }

  async findAll(tenant_id: string): Promise<User[]> {
    const { rows } = await this.pool.query(
      `SELECT * FROM users WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at ASC`,
      [tenant_id],
    )
    return rows.map(rowToUser)
  }

  async save(user: User): Promise<void> {
    await this.pool.query(
      `
      INSERT INTO users (
        sub, tenant_id, preferred_username, password_hash,
        name, given_name, family_name, email,
        email_verified, mfa_enrolled,
        roles, disabled_at,
        created_at, updated_at, deleted_at
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
      ON CONFLICT (sub) DO UPDATE SET
        tenant_id          = EXCLUDED.tenant_id,
        preferred_username = EXCLUDED.preferred_username,
        password_hash      = EXCLUDED.password_hash,
        name               = EXCLUDED.name,
        given_name         = EXCLUDED.given_name,
        family_name        = EXCLUDED.family_name,
        email              = EXCLUDED.email,
        email_verified     = EXCLUDED.email_verified,
        mfa_enrolled       = EXCLUDED.mfa_enrolled,
        roles              = EXCLUDED.roles,
        disabled_at        = EXCLUDED.disabled_at,
        updated_at         = EXCLUDED.updated_at,
        deleted_at         = EXCLUDED.deleted_at
      `,
      [
        user.sub,
        user.tenant_id,
        user.preferred_username,
        user.password_hash,
        user.name ?? null,
        user.given_name ?? null,
        user.family_name ?? null,
        user.email ?? null,
        user.email_verified,
        user.mfa_enrolled,
        JSON.stringify(user.roles),
        user.disabled_at ?? null,
        user.created_at,
        user.updated_at,
        user.deleted_at ?? null,
      ],
    )
  }
}
