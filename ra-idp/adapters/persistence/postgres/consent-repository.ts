/**
 * Layer 4 — Adapter Layer (Postgres ConsentRepository)
 *
 * (tenant_id, sub, client_id) を複合 PK とする consents テーブルを操作 (ADR-034)。
 */

import { ConsentSchema, type Consent } from '../../../src/spec-bindings/schemas'
import type { ConsentRepository } from '../../../src/oauth2/ports/consent-repository'
import type { PgPool } from './pool'

function rowToConsent(row: any): Consent {
  return ConsentSchema.parse({
    tenant_id: row.tenant_id,
    sub: row.sub,
    client_id: row.client_id,
    scopes: row.scopes,
    granted_at: row.granted_at instanceof Date ? row.granted_at.toISOString() : row.granted_at,
    expires_at: row.expires_at instanceof Date ? row.expires_at.toISOString() : row.expires_at,
    revoked_at: row.revoked_at
      ? row.revoked_at instanceof Date
        ? row.revoked_at.toISOString()
        : row.revoked_at
      : undefined,
  })
}

export class PostgresConsentRepository implements ConsentRepository {
  constructor(private readonly pool: PgPool) {}

  async find(tenant_id: string, sub: string, client_id: string): Promise<Consent | null> {
    const { rows } = await this.pool.query(
      `SELECT * FROM consents WHERE tenant_id = $1 AND sub = $2 AND client_id = $3`,
      [tenant_id, sub, client_id],
    )
    return rows[0] ? rowToConsent(rows[0]) : null
  }

  async save(consent: Consent): Promise<void> {
    await this.pool.query(
      `
      INSERT INTO consents (tenant_id, sub, client_id, scopes, granted_at, expires_at, revoked_at)
      VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)
      ON CONFLICT (tenant_id, sub, client_id) DO UPDATE SET
        scopes     = EXCLUDED.scopes,
        granted_at = EXCLUDED.granted_at,
        expires_at = EXCLUDED.expires_at,
        revoked_at = EXCLUDED.revoked_at
      `,
      [
        consent.tenant_id,
        consent.sub,
        consent.client_id,
        JSON.stringify(consent.scopes),
        consent.granted_at,
        consent.expires_at,
        consent.revoked_at ?? null,
      ],
    )
  }

  async revoke(tenant_id: string, sub: string, client_id: string): Promise<void> {
    await this.pool.query(
      `UPDATE consents SET revoked_at = now() WHERE tenant_id = $1 AND sub = $2 AND client_id = $3 AND revoked_at IS NULL`,
      [tenant_id, sub, client_id],
    )
  }
}
