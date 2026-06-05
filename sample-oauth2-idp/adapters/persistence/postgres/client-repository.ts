/**
 * Layer 4 — Adapter Layer (Postgres ClientRepository)
 *
 * infra/migrations/0001_init.sql の `clients` テーブルに対する CRUD。
 * 行 → Client への変換は schemas.ts (Zod) で検証することで、
 * spec/scl.yaml models.Client（実体は ClientRegistrationRequest 由来）との整合性を実行時に強制する。
 */

import { ClientSchema, type Client } from '../../../src/spec-bindings/schemas'
import type { ClientRepository } from '../../../src/ports/client-repository'
import type { PgPool } from './pool'

function rowToClient(row: any): Client {
  return ClientSchema.parse({
    client_id: row.client_id,
    client_secret_hash: row.client_secret_hash ?? undefined,
    client_name: row.client_name ?? undefined,
    client_type: row.client_type,
    redirect_uris: row.redirect_uris,
    grant_types: row.grant_types,
    response_types: row.response_types ?? [],
    token_endpoint_auth_method: row.token_endpoint_auth_method,
    scope: row.scope,
    jwks_uri: row.jwks_uri ?? undefined,
    jwks: row.jwks ?? undefined,
    tls_client_auth_subject_dn: row.tls_client_auth_subject_dn ?? undefined,
    id_token_signed_response_alg: row.id_token_signed_response_alg,
    require_pushed_authorization_requests: row.require_pushed_authorization_requests,
    dpop_bound_access_tokens: row.dpop_bound_access_tokens,
    fapi_profile: row.fapi_profile,
    created_at: row.created_at instanceof Date ? row.created_at.toISOString() : row.created_at,
  })
}

export class PostgresClientRepository implements ClientRepository {
  constructor(private readonly pool: PgPool) {}

  async findById(client_id: string): Promise<Client | null> {
    const { rows } = await this.pool.query(`SELECT * FROM clients WHERE client_id = $1`, [
      client_id,
    ])
    return rows[0] ? rowToClient(rows[0]) : null
  }

  async save(client: Client): Promise<void> {
    // 加法的変更を意図しているため UPSERT で挙動を統一する。
    // ADR-016: 既存の client_secret_hash を破壊しないよう、
    // 入力に未指定の値はそのまま保持する。
    await this.pool.query(
      `
      INSERT INTO clients (
        client_id, client_secret_hash, client_name, client_type,
        redirect_uris, grant_types, response_types,
        token_endpoint_auth_method, scope,
        jwks_uri, jwks, tls_client_auth_subject_dn,
        id_token_signed_response_alg,
        require_pushed_authorization_requests,
        dpop_bound_access_tokens, fapi_profile,
        created_at
      ) VALUES (
        $1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9,
        $10, $11::jsonb, $12, $13, $14, $15, $16, $17
      )
      ON CONFLICT (client_id) DO UPDATE SET
        client_secret_hash = COALESCE(EXCLUDED.client_secret_hash, clients.client_secret_hash),
        client_name = EXCLUDED.client_name,
        client_type = EXCLUDED.client_type,
        redirect_uris = EXCLUDED.redirect_uris,
        grant_types = EXCLUDED.grant_types,
        response_types = EXCLUDED.response_types,
        token_endpoint_auth_method = EXCLUDED.token_endpoint_auth_method,
        scope = EXCLUDED.scope,
        jwks_uri = EXCLUDED.jwks_uri,
        jwks = EXCLUDED.jwks,
        tls_client_auth_subject_dn = EXCLUDED.tls_client_auth_subject_dn,
        id_token_signed_response_alg = EXCLUDED.id_token_signed_response_alg,
        require_pushed_authorization_requests = EXCLUDED.require_pushed_authorization_requests,
        dpop_bound_access_tokens = EXCLUDED.dpop_bound_access_tokens,
        fapi_profile = EXCLUDED.fapi_profile
      `,
      [
        client.client_id,
        client.client_secret_hash ?? null,
        client.client_name ?? null,
        client.client_type,
        JSON.stringify(client.redirect_uris),
        JSON.stringify(client.grant_types),
        JSON.stringify(client.response_types),
        client.token_endpoint_auth_method,
        client.scope,
        client.jwks_uri ?? null,
        client.jwks ? JSON.stringify(client.jwks) : null,
        client.tls_client_auth_subject_dn ?? null,
        client.id_token_signed_response_alg,
        client.require_pushed_authorization_requests,
        client.dpop_bound_access_tokens,
        client.fapi_profile,
        client.created_at,
      ],
    )
  }

  async delete(client_id: string): Promise<void> {
    await this.pool.query(`DELETE FROM clients WHERE client_id = $1`, [client_id])
  }

  async findAll(): Promise<Client[]> {
    const { rows } = await this.pool.query(`SELECT * FROM clients ORDER BY created_at`)
    return rows.map(rowToClient)
  }
}
