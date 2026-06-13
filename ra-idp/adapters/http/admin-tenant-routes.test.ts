/**
 * Layer 4 — Adapter Layer (HTTP: /admin/tenants 統合テスト)
 *
 * ADR-032 §6 の control plane 認可境界を確認する。
 * - system_admin かつ default テナント所属でないと 403
 * - CRUD の正常系 + Tenant{Created,Updated,Disabled,Enabled} emit
 * - default テナントの disable は 400 (invariant)
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryTenantRepository } from '../persistence/memory/tenant-repository'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import {
  DEFAULT_TENANT_ID,
  TenantSchema,
  UserSchema,
  type DomainEvent,
} from '../../src/spec-bindings/schemas'
import { createCsrfToken, csrfCookie } from '../../src/shared/web-security'

import { createAdminTenantRoutes } from './admin-tenant-routes'

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const tenantRepo = new InMemoryTenantRepository()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)

  await tenantRepo.save(
    TenantSchema.parse({
      id: DEFAULT_TENANT_ID,
      display_name: 'Default',
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    }),
  )

  await userRepo.save(
    UserSchema.parse({
      sub: 'sys-admin',
      tenant_id: DEFAULT_TENANT_ID,
      preferred_username: 'sys',
      password_hash: 'pw',
      email: 'sys@example.com',
      email_verified: true,
      mfa_enrolled: false,
      roles: ['system_admin'],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  await userRepo.save(
    UserSchema.parse({
      sub: 'plain-user',
      tenant_id: DEFAULT_TENANT_ID,
      preferred_username: 'plain',
      password_hash: 'pw',
      email: 'plain@example.com',
      email_verified: true,
      mfa_enrolled: false,
      roles: [],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )

  const app = new Hono()
  app.route('/', createAdminTenantRoutes({ sessionManager, userRepo, tenantRepo, emit }))
  return { app, sessionManager, tenantRepo, events }
}

async function sessionFor(sm: LoginSessionManager, sub: string): Promise<string> {
  const ctx = await sm.create(DEFAULT_TENANT_ID, sub, ['pwd'], new Date())
  return `ra_idp_session=${ctx.session_id}`
}

function withCsrf(cookie: string): { cookies: string; csrf: string } {
  const csrf = createCsrfToken()
  return { csrf, cookies: `${cookie}; ${csrfCookie(csrf)}` }
}

describe('GET /admin/tenants', () => {
  it('未認証は 401', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.example.com/admin/tenants')
    expect(res.status).toBe(401)
  })

  it('system_admin でないユーザーは 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'plain-user')
    const res = await app.request('http://idp.example.com/admin/tenants', {
      headers: { cookie },
    })
    expect(res.status).toBe(403)
  })

  it('system_admin は一覧を取得できる', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const res = await app.request('http://idp.example.com/admin/tenants', {
      headers: { cookie },
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as { tenants: Array<{ id: string }> }
    expect(body.tenants.map((t) => t.id)).toContain(DEFAULT_TENANT_ID)
  })
})

describe('POST /admin/tenants (create)', () => {
  it('CSRF 不一致は 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const { cookies } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/tenants', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': 'wrong' },
      body: JSON.stringify({ id: 'acme', display_name: 'Acme' }),
    })
    expect(res.status).toBe(403)
  })

  it('正常系: 作成し TenantCreated を emit する', async () => {
    const { app, sessionManager, events, tenantRepo } = await setup()
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/tenants', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
      body: JSON.stringify({ id: 'acme', display_name: 'Acme' }),
    })
    expect(res.status).toBe(201)
    const persisted = await tenantRepo.findById('acme')
    expect(persisted?.display_name).toBe('Acme')
    expect(events.some((e) => e.type === 'TenantCreated')).toBe(true)
  })

  it('重複 id は 409', async () => {
    const { app, sessionManager, tenantRepo } = await setup()
    await tenantRepo.save(
      TenantSchema.parse({
        id: 'acme',
        display_name: 'Acme',
        status: 'active',
        created_at: '2024-01-01T00:00:00.000Z',
      }),
    )
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/tenants', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
      body: JSON.stringify({ id: 'acme', display_name: 'Acme 2' }),
    })
    expect(res.status).toBe(409)
  })
})

describe('POST /admin/tenants/:id/disable', () => {
  it('default テナントは disable できない (400)', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request(
      `http://idp.example.com/admin/tenants/${DEFAULT_TENANT_ID}/disable`,
      {
        method: 'POST',
        headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
      },
    )
    expect(res.status).toBe(400)
  })

  it('正常系: disable → enable で TenantDisabled / TenantEnabled が emit される', async () => {
    const { app, sessionManager, events, tenantRepo } = await setup()
    await tenantRepo.save(
      TenantSchema.parse({
        id: 'acme',
        display_name: 'Acme',
        status: 'active',
        created_at: '2024-01-01T00:00:00.000Z',
      }),
    )
    const cookie = await sessionFor(sessionManager, 'sys-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const disable = await app.request('http://idp.example.com/admin/tenants/acme/disable', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
    })
    expect(disable.status).toBe(204)
    const enable = await app.request('http://idp.example.com/admin/tenants/acme/enable', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
    })
    expect(enable.status).toBe(204)
    const kinds = events.map((e) => e.type)
    expect(kinds).toContain('TenantDisabled')
    expect(kinds).toContain('TenantEnabled')
  })
})
