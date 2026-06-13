/**
 * Layer 4 — Adapter Layer (Postgres TenantRepository)
 *
 * tenants テーブル (infra/migrations/0007_tenants.sql) を CRUD する。
 */

import { TenantSchema, type Tenant } from '../../../src/spec-bindings/schemas'
import type { TenantRepository } from '../../../src/tenancy/ports/tenant-repository'
import type { PgPool } from './pool'

function toIso(value: unknown): string | undefined {
  if (!value) return undefined
  return value instanceof Date ? value.toISOString() : String(value)
}

function rowToTenant(row: any): Tenant {
  return TenantSchema.parse({
    id: row.id,
    display_name: row.display_name,
    status: row.status,
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
    updated_at: toIso(row.updated_at),
    disabled_at: toIso(row.disabled_at),
  })
}

export class PostgresTenantRepository implements TenantRepository {
  constructor(private readonly pool: PgPool) {}

  async findById(id: string): Promise<Tenant | null> {
    const { rows } = await this.pool.query(`SELECT * FROM tenants WHERE id = $1`, [id])
    return rows[0] ? rowToTenant(rows[0]) : null
  }

  async findAll(): Promise<Tenant[]> {
    const { rows } = await this.pool.query(`SELECT * FROM tenants ORDER BY id`)
    return rows.map(rowToTenant)
  }

  async save(tenant: Tenant): Promise<void> {
    await this.pool.query(
      `
      INSERT INTO tenants (id, display_name, status, created_at, updated_at, disabled_at)
      VALUES ($1, $2, $3, $4, $5, $6)
      ON CONFLICT (id) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        status       = EXCLUDED.status,
        updated_at   = EXCLUDED.updated_at,
        disabled_at  = EXCLUDED.disabled_at
      `,
      [
        tenant.id,
        tenant.display_name,
        tenant.status,
        tenant.created_at,
        tenant.updated_at ?? null,
        tenant.disabled_at ?? null,
      ],
    )
  }
}
