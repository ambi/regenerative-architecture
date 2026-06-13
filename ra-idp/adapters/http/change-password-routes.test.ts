/**
 * Layer 4 — Adapter Layer (HTTP: /account/password 統合テスト)
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { createChangePasswordRoutes } from './change-password-routes'
import { InMemoryPasswordHistoryRepository } from '../persistence/memory/password-history-repo'
import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import { NoopBreachedPasswordChecker } from '../policy/noop-breached-password-checker'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import { UserSchema, type DomainEvent } from '../../src/spec-bindings/schemas'

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const passwordHistoryRepo = new InMemoryPasswordHistoryRepository()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const passwordHasher = new Argon2idPasswordHasher(8, 1)
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)

  await userRepo.save(
    UserSchema.parse({
      sub: 'user-alice',
      preferred_username: 'alice',
      password_hash: await passwordHasher.hash('current-password-1'),
      email_verified: true,
      mfa_enrolled: false,
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )

  const app = new Hono()
  app.route(
    '/',
    createChangePasswordRoutes({
      sessionManager,
      userRepo,
      passwordHasher,
      passwordHistoryRepo,
      breachedPasswordChecker: new NoopBreachedPasswordChecker(),
      emit,
    }),
  )

  return { app, sessionManager, userRepo, passwordHasher, events }
}

async function authenticatedSession(
  sessionManager: LoginSessionManager,
): Promise<{ cookieHeader: string }> {
  const ctx = await sessionManager.create('user-alice', ['pwd'], new Date())
  return { cookieHeader: `ra_idp_session=${ctx.session_id}` }
}

function extractMeta(html: string, name: string): string {
  const m = html.match(new RegExp(`name="ra-idp:${name}" content="([^"]+)"`))
  if (!m) throw new Error(`meta ${name} not found`)
  return m[1]
}

describe('GET /account/password', () => {
  it('未ログインなら /login へ 303', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.example.com/account/password')
    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/login')
  })

  it('ログイン済みなら shell + CSRF cookie を返す', async () => {
    const { app, sessionManager } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const res = await app.request('http://idp.example.com/account/password', {
      headers: { cookie: cookieHeader },
    })
    expect(res.status).toBe(200)
    const setCookie = res.headers.get('set-cookie') ?? ''
    expect(setCookie).toContain('ra_idp_csrf=')
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="change-password"')
  })

  it('authentication_pending なら /login へ 303 (TOTP 未完了)', async () => {
    const { app, sessionManager } = await setup()
    const ctx = await sessionManager.create('user-alice', ['pwd'], new Date(), {
      authenticationPending: true,
    })
    const res = await app.request('http://idp.example.com/account/password', {
      headers: { cookie: `ra_idp_session=${ctx.session_id}` },
    })
    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/login')
  })
})

async function getCsrf(
  app: Hono,
  cookieHeader: string,
): Promise<{ csrf: string; cookies: string }> {
  const res = await app.request('http://idp.example.com/account/password', {
    headers: { cookie: cookieHeader },
  })
  const html = await res.text()
  const csrf = extractMeta(html, 'csrf')
  const csrfCookie = res.headers.get('set-cookie') ?? ''
  return { csrf, cookies: `${cookieHeader}; ${csrfCookie}` }
}

describe('POST /api/auth/change_password', () => {
  it('正常系: 成功 200 + PasswordChanged emit + 履歴に追加', async () => {
    const { app, sessionManager, userRepo, passwordHasher, events } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const { csrf, cookies } = await getCsrf(app, cookieHeader)

    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({
        current_password: 'current-password-1',
        new_password: 'fresh-password-9182',
      }),
    })
    expect(res.status).toBe(200)
    expect(await res.json()).toEqual({ status: 'ok' })

    const updated = await userRepo.findBySub('user-alice')
    expect(await passwordHasher.verify('fresh-password-9182', updated!.password_hash)).toBe(true)
    expect(events.some((e) => e.type === 'PasswordChanged')).toBe(true)
  })

  it('CSRF 不一致は 403', async () => {
    const { app, sessionManager } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const { cookies } = await getCsrf(app, cookieHeader)

    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': 'bad',
        cookie: cookies,
      },
      body: JSON.stringify({
        current_password: 'current-password-1',
        new_password: 'fresh-password-9182',
      }),
    })
    expect(res.status).toBe(403)
    expect(await res.json()).toMatchObject({ error: 'csrf_failed' })
  })

  it('セッションなしは 401', async () => {
    const { app } = await setup()
    // CSRF cookie だけ用意するために GET (未認証で 303 だが set-cookie はある) は使えないので、
    // 直接 cookie を組み立てる。
    const csrfBypass = 'csrf-x'
    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrfBypass,
        cookie: `ra_idp_csrf=${csrfBypass}`,
      },
      body: JSON.stringify({
        current_password: 'current-password-1',
        new_password: 'fresh-password-9182',
      }),
    })
    expect(res.status).toBe(401)
    expect(await res.json()).toMatchObject({ error: 'session_required' })
  })

  it('現パス不一致は 400 current_password_mismatch', async () => {
    const { app, sessionManager, events } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const { csrf, cookies } = await getCsrf(app, cookieHeader)

    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({
        current_password: 'wrong',
        new_password: 'fresh-password-9182',
      }),
    })
    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'current_password_mismatch' })
    expect(events.some((e) => e.type === 'PasswordChanged')).toBe(false)
  })

  it('新パス policy 違反は 400 password_policy_violation', async () => {
    const { app, sessionManager } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const { csrf, cookies } = await getCsrf(app, cookieHeader)

    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({
        current_password: 'current-password-1',
        new_password: 'short',
      }),
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string; violations: string[] }
    expect(body.error).toBe('password_policy_violation')
    expect(body.violations).toContain('too_short')
  })

  it('履歴再利用は 400 password_reuse', async () => {
    const { app, sessionManager } = await setup()
    const { cookieHeader } = await authenticatedSession(sessionManager)
    const { csrf, cookies } = await getCsrf(app, cookieHeader)

    const res = await app.request('http://idp.example.com/api/auth/change_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': csrf,
        cookie: cookies,
      },
      body: JSON.stringify({
        current_password: 'current-password-1',
        new_password: 'current-password-1',
      }),
    })
    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'password_reuse' })
  })
})
