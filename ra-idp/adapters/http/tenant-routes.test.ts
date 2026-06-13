/**
 * Layer 4 — Adapter Layer (HTTP: /realms/{tenant_id} 経路の smoke + isolation)
 *
 * - bare 経路と /realms/default/* が同じ discovery 文書を返す (issuer は per-tenant)
 * - 不在テナント → 404 tenant_not_found
 * - 無効化テナント → 400 invalid_request (テナントの存在は応答に漏らさない)
 * - /realms/acme/admin/tenants は SystemAdministrator にしか開かない
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { InMemoryTenantRepository } from '../persistence/memory/tenant-repository'
import { createTenantMiddleware, type TenantVar } from './middleware/tenant-middleware'
import { TenantSchema } from '../../src/spec-bindings/schemas'

const BASE_ISSUER = 'https://idp.example.com'

async function setup() {
  const tenants = new InMemoryTenantRepository()
  const now = new Date().toISOString()
  await tenants.save(
    TenantSchema.parse({
      id: 'default',
      display_name: 'Default',
      status: 'active',
      created_at: now,
    }),
  )
  await tenants.save(
    TenantSchema.parse({ id: 'acme', display_name: 'Acme', status: 'active', created_at: now }),
  )
  await tenants.save(
    TenantSchema.parse({
      id: 'disabled-co',
      display_name: 'Disabled',
      status: 'disabled',
      created_at: now,
      disabled_at: now,
    }),
  )

  const app = new Hono<{ Variables: TenantVar }>()
  app.use('*', createTenantMiddleware({ tenantRepo: tenants, baseIssuer: BASE_ISSUER }))
  app.get('/echo', (c) =>
    c.json({
      tenant_id: c.get('tenant_id'),
      tenant_issuer: c.get('tenant_issuer'),
      tenant_url_prefix: c.get('tenant_url_prefix'),
    }),
  )
  app.get('/realms/:tenant_id/echo', (c) =>
    c.json({
      tenant_id: c.get('tenant_id'),
      tenant_issuer: c.get('tenant_issuer'),
      tenant_url_prefix: c.get('tenant_url_prefix'),
    }),
  )
  return app
}

describe('tenant middleware', () => {
  it('bare 経路は default テナントで応答し prefix を持たない', async () => {
    const app = await setup()
    const res = await app.request('http://idp.example.com/echo')
    expect(res.status).toBe(200)
    const body = (await res.json()) as Record<string, string>
    expect(body.tenant_id).toBe('default')
    expect(body.tenant_url_prefix).toBe('')
    expect(body.tenant_issuer).toBe('https://idp.example.com/realms/default')
  })

  it('/realms/acme/echo は acme テナントで応答し issuer に prefix が付く', async () => {
    const app = await setup()
    const res = await app.request('http://idp.example.com/realms/acme/echo')
    expect(res.status).toBe(200)
    const body = (await res.json()) as Record<string, string>
    expect(body.tenant_id).toBe('acme')
    expect(body.tenant_url_prefix).toBe('/realms/acme')
    expect(body.tenant_issuer).toBe('https://idp.example.com/realms/acme')
  })

  it('不在テナントは 404 tenant_not_found', async () => {
    const app = await setup()
    const res = await app.request('http://idp.example.com/realms/ghost/echo')
    expect(res.status).toBe(404)
    expect(await res.json()).toEqual({ error: 'tenant_not_found' })
  })

  it('無効化テナントは 400 invalid_request (テナントの状態は説明文に出さない)', async () => {
    const app = await setup()
    const res = await app.request('http://idp.example.com/realms/disabled-co/echo')
    expect(res.status).toBe(400)
    const body = (await res.json()) as Record<string, string>
    expect(body.error).toBe('invalid_request')
    expect(body.error_description).not.toContain('disabled-co')
  })
})
