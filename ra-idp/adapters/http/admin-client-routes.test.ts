/**
 * Layer 4 — Adapter Layer (HTTP: /admin/clients 統合テスト)
 *
 * Mirrors ra-idp-go/internal/adapters/http/admin_client_handler_test.go.
 * テナント境界 (admin の tenant_id とリクエストの tenant_id が一致しないと 403)
 * を含めて create/get/list/update/delete を回す。
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { InMemoryClientRepository } from '../persistence/memory/client-repo'
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

import { createAdminClientRoutes } from './admin-client-routes'
import { createTenantMiddleware } from './middleware/tenant-middleware'

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const clientRepo = new InMemoryClientRepository()
  const tenantRepo = new InMemoryTenantRepository()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)

  for (const id of [DEFAULT_TENANT_ID, 'acme']) {
    await tenantRepo.save(
      TenantSchema.parse({
        id,
        display_name: id,
        status: 'active',
        created_at: '2024-01-01T00:00:00.000Z',
      }),
    )
  }

  await userRepo.save(
    UserSchema.parse({
      sub: 'default-admin',
      tenant_id: DEFAULT_TENANT_ID,
      preferred_username: 'da',
      password_hash: 'pw',
      email: 'da@example.com',
      email_verified: true,
      mfa_enrolled: false,
      roles: ['admin'],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  await userRepo.save(
    UserSchema.parse({
      sub: 'acme-admin',
      tenant_id: 'acme',
      preferred_username: 'aa',
      password_hash: 'pw',
      email: 'aa@example.com',
      email_verified: true,
      mfa_enrolled: false,
      roles: ['admin'],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )

  const app = new Hono()
  app.use(
    '*',
    createTenantMiddleware({ tenantRepo, baseIssuer: 'https://idp.example.com' }),
  )
  app.route('/', createAdminClientRoutes({ sessionManager, userRepo, clientRepo, emit }))
  app.route('/realms/:tenant_id', createAdminClientRoutes({ sessionManager, userRepo, clientRepo, emit }))

  return { app, sessionManager, clientRepo, tenantRepo, events }
}

async function sessionFor(sm: LoginSessionManager, tenantId: string, sub: string): Promise<string> {
  const ctx = await sm.create(tenantId, sub, ['pwd'], new Date())
  return `ra_idp_session=${ctx.session_id}`
}

function withCsrf(cookie: string): { cookies: string; csrf: string } {
  const csrf = createCsrfToken()
  return { csrf, cookies: `${cookie}; ${csrfCookie(csrf)}` }
}

describe('GET /admin/clients (list)', () => {
  it('未認証は 401', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.example.com/admin/clients')
    expect(res.status).toBe(401)
  })

  it('admin は自テナント のクライアントだけ返す', async () => {
    const { app, sessionManager, clientRepo } = await setup()
    await clientRepo.save({
      tenant_id: DEFAULT_TENANT_ID,
      client_id: 'web-default',
      client_type: 'public',
      redirect_uris: ['https://w/cb'],
      grant_types: ['authorization_code'],
      response_types: ['code'],
      token_endpoint_auth_method: 'none',
      scope: 'openid',
      created_at: '2024-01-01T00:00:00.000Z',
    } as unknown as Parameters<typeof clientRepo.save>[0])
    await clientRepo.save({
      tenant_id: 'acme',
      client_id: 'web-acme',
      client_type: 'public',
      redirect_uris: ['https://a/cb'],
      grant_types: ['authorization_code'],
      response_types: ['code'],
      token_endpoint_auth_method: 'none',
      scope: 'openid',
      created_at: '2024-01-01T00:00:00.000Z',
    } as unknown as Parameters<typeof clientRepo.save>[0])

    const cookie = await sessionFor(sessionManager, DEFAULT_TENANT_ID, 'default-admin')
    const res = await app.request('http://idp.example.com/admin/clients', {
      headers: { cookie },
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as { clients: Array<{ client_id: string }> }
    expect(body.clients.map((c) => c.client_id)).toEqual(['web-default'])
  })

  it('別テナントの admin (default 経路) は 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'acme', 'acme-admin')
    const res = await app.request('http://idp.example.com/admin/clients', {
      headers: { cookie },
    })
    expect(res.status).toBe(403)
  })

  it('acme tenant prefix で acme-admin が acme クライアントを取得できる', async () => {
    const { app, sessionManager, clientRepo } = await setup()
    await clientRepo.save({
      tenant_id: 'acme',
      client_id: 'web-acme',
      client_type: 'public',
      redirect_uris: ['https://a/cb'],
      grant_types: ['authorization_code'],
      response_types: ['code'],
      token_endpoint_auth_method: 'none',
      scope: 'openid',
      created_at: '2024-01-01T00:00:00.000Z',
    } as unknown as Parameters<typeof clientRepo.save>[0])
    const cookie = await sessionFor(sessionManager, 'acme', 'acme-admin')
    const res = await app.request('http://idp.example.com/realms/acme/admin/clients', {
      headers: { cookie },
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as { clients: Array<{ client_id: string }> }
    expect(body.clients.map((c) => c.client_id)).toEqual(['web-acme'])
  })
})

describe('POST /admin/clients (create)', () => {
  it('CSRF 不一致は 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, DEFAULT_TENANT_ID, 'default-admin')
    const { cookies } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/clients', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': 'wrong' },
      body: JSON.stringify({
        client_type: 'public',
        redirect_uris: ['https://x/cb'],
        token_endpoint_auth_method: 'none',
      }),
    })
    expect(res.status).toBe(403)
  })

  it('正常系: 作成し AdminClientCreated を emit する', async () => {
    const { app, sessionManager, events, clientRepo } = await setup()
    const cookie = await sessionFor(sessionManager, DEFAULT_TENANT_ID, 'default-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/clients', {
      method: 'POST',
      headers: { 'content-type': 'application/json', cookie: cookies, 'X-CSRF-Token': csrf },
      body: JSON.stringify({
        client_type: 'public',
        redirect_uris: ['https://x/cb'],
        token_endpoint_auth_method: 'none',
        grant_types: ['authorization_code'],
        response_types: ['code'],
      }),
    })
    expect(res.status).toBe(201)
    const body = (await res.json()) as { client: { client_id: string; tenant_id: string } }
    expect(body.client.tenant_id).toBe(DEFAULT_TENANT_ID)
    const stored = await clientRepo.findById(DEFAULT_TENANT_ID, body.client.client_id)
    expect(stored).not.toBeNull()
    expect(events.some((e) => e.type === 'AdminClientCreated')).toBe(true)
  })
})

describe('DELETE /admin/clients/:id', () => {
  it('正常系: 削除し AdminClientDeleted を emit する', async () => {
    const { app, sessionManager, events, clientRepo } = await setup()
    await clientRepo.save({
      tenant_id: DEFAULT_TENANT_ID,
      client_id: 'to-delete',
      client_type: 'public',
      redirect_uris: ['https://x/cb'],
      grant_types: ['authorization_code'],
      response_types: ['code'],
      token_endpoint_auth_method: 'none',
      scope: 'openid',
      created_at: '2024-01-01T00:00:00.000Z',
    } as unknown as Parameters<typeof clientRepo.save>[0])
    const cookie = await sessionFor(sessionManager, DEFAULT_TENANT_ID, 'default-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/clients/to-delete', {
      method: 'DELETE',
      headers: { cookie: cookies, 'X-CSRF-Token': csrf },
    })
    expect(res.status).toBe(204)
    expect(await clientRepo.findById(DEFAULT_TENANT_ID, 'to-delete')).toBeNull()
    expect(events.some((e) => e.type === 'AdminClientDeleted')).toBe(true)
  })

  it('存在しない client は 404', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, DEFAULT_TENANT_ID, 'default-admin')
    const { cookies, csrf } = withCsrf(cookie)
    const res = await app.request('http://idp.example.com/admin/clients/missing', {
      method: 'DELETE',
      headers: { cookie: cookies, 'X-CSRF-Token': csrf },
    })
    expect(res.status).toBe(404)
  })
})
