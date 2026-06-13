/**
 * Layer 4 — Adapter Layer (HTTP: /admin/users 統合テスト)
 *
 * ADR-031 の RBAC + CSRF 境界と、create/update/disable の正常系 + UserCreated /
 * UserUpdated / UserDisabled / UserEnabled emit を回す。
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import { InMemoryPasswordHistoryRepository } from '../persistence/memory/password-history-repo'
import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import { UserSchema, type DomainEvent } from '../../src/spec-bindings/schemas'

import { createAdminUserRoutes } from './admin-user-routes'

const HASHER = new Argon2idPasswordHasher(8, 1)

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const passwordHistoryRepo = new InMemoryPasswordHistoryRepository()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)

  await userRepo.save(
    UserSchema.parse({
      sub: 'user-admin',
      tenant_id: 'default',
      preferred_username: 'operator',
      password_hash: await HASHER.hash('operator-password-1'),
      email: 'operator@example.com',
      email_verified: true,
      mfa_enrolled: false,
      roles: ['admin'],
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  await userRepo.save(
    UserSchema.parse({
      sub: 'user-bob',
      tenant_id: 'default',
      preferred_username: 'bob',
      password_hash: await HASHER.hash('bob-password-12345'),
      email: 'bob@example.com',
      email_verified: false,
      mfa_enrolled: false,
      roles: [],
      created_at: '2024-01-02T00:00:00.000Z',
      updated_at: '2024-01-02T00:00:00.000Z',
    }),
  )

  const app = new Hono()
  app.route(
    '/',
    createAdminUserRoutes({
      sessionManager,
      userRepo,
      passwordHasher: HASHER,
      passwordHistoryRepo,
      emit,
    }),
  )

  return { app, sessionManager, userRepo, events }
}

async function sessionFor(sessionManager: LoginSessionManager, sub: string): Promise<string> {
  const ctx = await sessionManager.create('default', sub, ['pwd'], new Date())
  return `ra_idp_session=${ctx.session_id}`
}

async function csrfFor(
  app: Hono,
  cookieHeader: string,
): Promise<{ csrf: string; cookies: string }> {
  const res = await app.request('http://idp.example.com/admin/users', {
    headers: { cookie: cookieHeader },
  })
  expect(res.status).toBe(200)
  const html = await res.text()
  const match = html.match(/name="ra-idp:csrf" content="([^"]+)"/)
  if (!match) throw new Error('csrf meta not found')
  const setCookie = res.headers.get('set-cookie') ?? ''
  return { csrf: match[1], cookies: `${cookieHeader}; ${setCookie}` }
}

describe('GET /admin/users (shell)', () => {
  it('未認証は /login へ 303', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.example.com/admin/users')
    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/login')
  })

  it('admin でないユーザーは 403 + error shell', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-bob')
    const res = await app.request('http://idp.example.com/admin/users', {
      headers: { cookie },
    })
    expect(res.status).toBe(403)
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="error"')
  })

  it('admin は shell + CSRF cookie を受け取る', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const res = await app.request('http://idp.example.com/admin/users', {
      headers: { cookie },
    })
    expect(res.status).toBe(200)
    expect(res.headers.get('set-cookie') ?? '').toContain('ra_idp_csrf=')
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="admin-users"')
  })
})

describe('GET /api/admin/users', () => {
  it('未認証は 401', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.example.com/api/admin/users')
    expect(res.status).toBe(401)
  })

  it('admin でないユーザーは 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-bob')
    const res = await app.request('http://idp.example.com/api/admin/users', {
      headers: { cookie },
    })
    expect(res.status).toBe(403)
  })

  it('admin は両ユーザーを返す', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const res = await app.request('http://idp.example.com/api/admin/users', {
      headers: { cookie },
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as { users: Array<{ sub: string; password_hash?: string }> }
    expect(body.users.map((u) => u.sub).sort()).toEqual(['user-admin', 'user-bob'])
    for (const u of body.users) {
      expect((u as Record<string, unknown>).password_hash).toBeUndefined()
    }
  })
})

describe('POST /api/admin/users', () => {
  it('CSRF 不一致は 403', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { cookies } = await csrfFor(app, cookie)
    const res = await app.request('http://idp.example.com/api/admin/users', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': 'wrong',
        cookie: cookies,
      },
      body: JSON.stringify({ preferred_username: 'carol', password: 'fresh-password-12345' }),
    })
    expect(res.status).toBe(403)
  })

  it('正常系: ユーザーを作成し UserCreated を emit', async () => {
    const { app, sessionManager, userRepo, events } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)

    const res = await app.request('http://idp.example.com/api/admin/users', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({
        preferred_username: 'carol',
        password: 'fresh-password-12345',
        email: 'carol@example.com',
        roles: ['support'],
      }),
    })
    expect(res.status).toBe(201)
    const body = (await res.json()) as { sub: string; preferred_username: string; roles: string[] }
    expect(body.preferred_username).toBe('carol')
    expect(body.roles).toEqual(['support'])

    const stored = await userRepo.findByUsername('carol')
    expect(stored).not.toBeNull()
    expect(stored?.roles).toEqual(['support'])

    const created = events.find((e) => e.type === 'UserCreated')
    expect(created).toBeDefined()
    expect((created as { actorSub: string }).actorSub).toBe('user-admin')
  })

  it('username 衝突は 409', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)
    const res = await app.request('http://idp.example.com/api/admin/users', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({ preferred_username: 'bob', password: 'fresh-password-123456' }),
    })
    expect(res.status).toBe(409)
    expect(await res.json()).toMatchObject({ error: 'username_conflict' })
  })

  it('password policy 違反は 400 + violations', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)
    const res = await app.request('http://idp.example.com/api/admin/users', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({ preferred_username: 'carol', password: 'short' }),
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string; violations: string[] }
    expect(body.error).toBe('password_policy')
    expect(body.violations).toContain('too_short')
  })
})

describe('PATCH /api/admin/users/:sub', () => {
  it('roles 更新で UserUpdated emit (changedFields=["roles"])', async () => {
    const { app, sessionManager, userRepo, events } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)

    const res = await app.request('http://idp.example.com/api/admin/users/user-bob', {
      method: 'PATCH',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({ roles: ['support'] }),
    })
    expect(res.status).toBe(200)

    const updated = await userRepo.findBySub('user-bob')
    expect(updated?.roles).toEqual(['support'])

    const event = events.find((e) => e.type === 'UserUpdated') as
      | { actorSub: string; targetSub: string; changedFields: string[] }
      | undefined
    expect(event).toBeDefined()
    expect(event?.changedFields).toEqual(['roles'])
    expect(event?.targetSub).toBe('user-bob')
  })

  it('存在しない sub は 404', async () => {
    const { app, sessionManager } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)
    const res = await app.request('http://idp.example.com/api/admin/users/missing', {
      method: 'PATCH',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({ name: 'Nope' }),
    })
    expect(res.status).toBe(404)
  })
})

describe('POST /api/admin/users/:sub/disable + enable', () => {
  it('disable で 204 + UserDisabled、enable で UserEnabled', async () => {
    const { app, sessionManager, userRepo, events } = await setup()
    const cookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, cookie)

    const disableRes = await app.request(
      'http://idp.example.com/api/admin/users/user-bob/disable',
      {
        method: 'POST',
        headers: { 'X-CSRF-Token': csrf, cookie: cookies },
      },
    )
    expect(disableRes.status).toBe(204)
    expect((await userRepo.findBySub('user-bob'))?.disabled_at).toBeDefined()
    expect(events.some((e) => e.type === 'UserDisabled')).toBe(true)

    const enableRes = await app.request('http://idp.example.com/api/admin/users/user-bob/enable', {
      method: 'POST',
      headers: { 'X-CSRF-Token': csrf, cookie: cookies },
    })
    expect(enableRes.status).toBe(204)
    expect((await userRepo.findBySub('user-bob'))?.disabled_at).toBeUndefined()
    expect(events.some((e) => e.type === 'UserEnabled')).toBe(true)
  })

  it('disabled admin は管理 API を呼べない (forbidden)', async () => {
    const { app, sessionManager, userRepo } = await setup()
    const adminCookie = await sessionFor(sessionManager, 'user-admin')
    const { csrf, cookies } = await csrfFor(app, adminCookie)

    // admin を直接 disable してロックアウトを模倣
    const admin = await userRepo.findBySub('user-admin')
    if (!admin) throw new Error('seed missing')
    await userRepo.save({ ...admin, disabled_at: '2024-02-01T00:00:00.000Z' })

    const res = await app.request('http://idp.example.com/api/admin/users', {
      headers: { cookie: cookies, 'X-CSRF-Token': csrf },
    })
    expect(res.status).toBe(403)
  })
})
