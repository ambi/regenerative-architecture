/**
 * Layer 4 — Adapter Layer (HTTP tenant resolver middleware)
 *
 * ADR-033: テナント解決は `/realms/{tenant_id}` パスプレフィックスで行う。
 * 未 prefix のリクエストは `default` テナントへフォールバックする (後方互換)。
 *
 * Hono の context (c.var) に Tenant + issuer + url_prefix を載せる。下流の
 * route handler はこれらを参照してテナント境界を尊重する (Go 側 tenancy/context.go
 * と双子)。
 */

import type { Context, MiddlewareHandler } from 'hono'

import { DEFAULT_TENANT_ID, type Tenant } from '../../../src/spec-bindings/schemas'
import type { TenantRepository } from '../../../src/tenancy/ports/tenant-repository'

export type TenantVar = {
  tenant: Tenant
  tenant_id: string
  tenant_issuer: string
  tenant_url_prefix: string
}

export interface TenantMiddlewareOptions {
  tenantRepo?: TenantRepository
  baseIssuer: string
  /** ADR-033 §3 escape hatch: default テナントの bare 経路で base URL をそのまま `iss` に使う。 */
  legacyBareIssuer?: boolean
}

const TENANT_PATTERN = /^\/realms\/([a-z0-9][a-z0-9-]{0,62})(?:\/|$)/

/**
 * パスから `/realms/{tenant_id}` を抜き出し、ctx に Tenant をセットする。
 * - 未 prefix → `default` (bare 互換)
 * - 不在テナント → 404 `tenant_not_found`
 * - 無効化テナント (protocol route) → 400 `invalid_request`
 */
export function createTenantMiddleware(opts: TenantMiddlewareOptions): MiddlewareHandler {
  const baseIssuer = opts.baseIssuer.replace(/\/+$/, '')

  return async (c, next) => {
    const match = c.req.path.match(TENANT_PATTERN)
    const tenantId = match ? match[1] : DEFAULT_TENANT_ID
    const bare = match === null
    const urlPrefix = bare ? '' : `/realms/${tenantId}`

    const tenant = await resolveTenant(opts.tenantRepo, tenantId)
    if (!tenant) {
      return c.json({ error: 'tenant_not_found' }, 404)
    }
    if (tenant.status !== 'active' || tenant.disabled_at) {
      return c.json({ error: 'invalid_request', error_description: 'tenant is unavailable' }, 400)
    }

    const issuer = bare && opts.legacyBareIssuer ? baseIssuer : `${baseIssuer}/realms/${tenantId}`

    c.set('tenant', tenant)
    c.set('tenant_id', tenant.id)
    c.set('tenant_issuer', issuer)
    c.set('tenant_url_prefix', urlPrefix)

    await next()
  }
}

async function resolveTenant(
  repo: TenantRepository | undefined,
  id: string,
): Promise<Tenant | null> {
  if (!repo) {
    if (id !== DEFAULT_TENANT_ID) return null
    // テナント永続化が未注入の (testing 等) フォールバック。
    return {
      id: DEFAULT_TENANT_ID,
      display_name: 'Default',
      status: 'active',
      created_at: new Date(0).toISOString(),
    }
  }
  return repo.findById(id)
}

/** 下流の handler から tenant id を読み出すヘルパー (型注釈用)。 */
export function requestTenantId(c: Context): string {
  return (c.get('tenant_id') as string | undefined) ?? DEFAULT_TENANT_ID
}

export function requestIssuer(c: Context, fallback: string): string {
  return (c.get('tenant_issuer') as string | undefined) ?? fallback.replace(/\/+$/, '')
}

export function tenantUrlPrefix(c: Context): string {
  return (c.get('tenant_url_prefix') as string | undefined) ?? ''
}

export function tenantRoute(c: Context, path: string): string {
  const prefix = tenantUrlPrefix(c)
  return prefix ? prefix + path : path
}

/**
 * Cookie の Path 属性 (`Set-Cookie; Path=...`)。
 * - bare 経路 → `/`
 * - `/realms/{id}` → `/realms/{id}`
 *
 * これにより tenant ごとに Cookie が分離され、ブラウザレベルでセッションが
 * 漏れないようになる (ra-idp-go と同じ戦略)。
 */
export function tenantCookiePath(c: Context): string {
  const prefix = tenantUrlPrefix(c)
  return prefix || '/'
}
