/**
 * Layer 4 — Adapter Layer (HTTP: /forgot_password / /reset_password 統合テスト)
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { createHash } from 'crypto'

import { createPasswordResetRoutes } from './password-reset-routes'
import { InMemoryPasswordHistoryRepository } from '../persistence/memory/password-history-repo'
import { InMemoryPasswordResetTokenStore } from '../persistence/memory/password-reset-token-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import { NoopBreachedPasswordChecker } from '../policy/noop-breached-password-checker'
import { NoopEmailSender } from '../notification/noop-email-sender'
import { UserSchema, type DomainEvent } from '../../src/spec-bindings/schemas'

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const passwordHistoryRepo = new InMemoryPasswordHistoryRepository()
  const passwordResetTokenStore = new InMemoryPasswordResetTokenStore()
  const passwordHasher = new Argon2idPasswordHasher(8, 1)
  const emailSender = new NoopEmailSender()
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)
  await userRepo.save(
    UserSchema.parse({
      sub: 'user-alice',
      tenant_id: 'default',
      preferred_username: 'alice',
      password_hash: await passwordHasher.hash('current-password-1'),
      email: 'alice@example.com',
      email_verified: true,
      mfa_enrolled: false,
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  const app = new Hono()
  app.route(
    '/',
    createPasswordResetRoutes({
      userRepo,
      passwordHasher,
      passwordHistoryRepo,
      passwordResetTokenStore,
      emailSender,
      breachedPasswordChecker: new NoopBreachedPasswordChecker(),
      emit,
      issuer: 'http://idp.test',
    }),
  )
  return {
    app,
    userRepo,
    passwordHistoryRepo,
    passwordResetTokenStore,
    passwordHasher,
    emailSender,
    events,
  }
}

function csrfPair() {
  const csrf = 'csrf-' + Math.random().toString(36).slice(2, 10)
  return { csrf, cookie: `ra_idp_csrf=${csrf}` }
}

describe('GET /forgot_password', () => {
  it('shell + CSRF cookie を返す', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.test/forgot_password')
    expect(res.status).toBe(200)
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="forgot-password"')
    expect(res.headers.get('set-cookie') ?? '').toContain('ra_idp_csrf=')
  })
})

describe('POST /api/auth/forgot_password', () => {
  it('正常系: 204 + email 送信 + イベント emit', async () => {
    const { app, emailSender, events } = await setup()
    const { csrf, cookie } = csrfPair()
    const res = await app.request('http://idp.test/api/auth/forgot_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ email: 'alice@example.com' }),
    })
    expect(res.status).toBe(204)
    expect(emailSender.sent).toHaveLength(1)
    expect(events.map((e) => e.type)).toContain('PasswordResetRequested')
    expect(events.map((e) => e.type)).toContain('EmailSent')
  })

  it('未登録 email でも 204 (anti-enumeration)', async () => {
    const { app, emailSender } = await setup()
    const { csrf, cookie } = csrfPair()
    const res = await app.request('http://idp.test/api/auth/forgot_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ email: 'ghost@example.com' }),
    })
    expect(res.status).toBe(204)
    expect(emailSender.sent).toHaveLength(0)
  })

  it('CSRF 不一致は 403', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.test/api/auth/forgot_password', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': 'wrong',
        cookie: 'ra_idp_csrf=expected',
      },
      body: JSON.stringify({ email: 'alice@example.com' }),
    })
    expect(res.status).toBe(403)
  })
})

describe('GET /reset_password', () => {
  it('shell + CSRF cookie + token meta を返す', async () => {
    const { app } = await setup()
    const res = await app.request('http://idp.test/reset_password?token=abc-123')
    expect(res.status).toBe(200)
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="reset-password"')
    expect(html).toContain('name="ra-idp:reset-token" content="abc-123"')
    expect(res.headers.get('set-cookie') ?? '').toContain('ra_idp_csrf=')
  })
})

async function issueResetToken(app: Hono): Promise<string> {
  const { csrf, cookie } = csrfPair()
  // forgot_password を呼んで token を発行
  await app.request('http://idp.test/api/auth/forgot_password', {
    method: 'POST',
    headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
    body: JSON.stringify({ email: 'alice@example.com' }),
  })
  return ''
}

describe('POST /api/auth/reset_password', () => {
  it('正常系: 200 + パスワード更新', async () => {
    const { app, emailSender, userRepo, passwordHasher } = await setup()
    await issueResetToken(app)
    const sentUrl = emailSender.sent[0].text
    const tokenMatch = sentUrl.match(/token=([A-Za-z0-9_-]+)/)
    const rawToken = decodeURIComponent(tokenMatch?.[1] ?? '')

    const { csrf, cookie } = csrfPair()
    const res = await app.request('http://idp.test/api/auth/reset_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ token: rawToken, new_password: 'fresh-pass-9182' }),
    })
    expect(res.status).toBe(200)
    const user = await userRepo.findBySub('user-alice')
    expect(await passwordHasher.verify('fresh-pass-9182', user!.password_hash)).toBe(true)
  })

  it('期限切れ / 不正 token は 410', async () => {
    const { app } = await setup()
    const { csrf, cookie } = csrfPair()
    const res = await app.request('http://idp.test/api/auth/reset_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ token: 'bogus', new_password: 'fresh-pass-9182' }),
    })
    expect(res.status).toBe(410)
    expect(await res.json()).toMatchObject({ error: 'invalid_reset_token' })
  })

  it('policy 違反は 400 password_policy_violation', async () => {
    const { app, emailSender } = await setup()
    await issueResetToken(app)
    const sentUrl = emailSender.sent[0].text
    const tokenMatch = sentUrl.match(/token=([A-Za-z0-9_-]+)/)
    const rawToken = decodeURIComponent(tokenMatch?.[1] ?? '')

    const { csrf, cookie } = csrfPair()
    const res = await app.request('http://idp.test/api/auth/reset_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ token: rawToken, new_password: 'short' }),
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string; violations: string[] }
    expect(body.error).toBe('password_policy_violation')
    expect(body.violations).toContain('too_short')
  })

  it('単発消費: 同じ token で 2 度目は 410', async () => {
    const { app, emailSender } = await setup()
    await issueResetToken(app)
    const sentUrl = emailSender.sent[0].text
    const tokenMatch = sentUrl.match(/token=([A-Za-z0-9_-]+)/)
    const rawToken = decodeURIComponent(tokenMatch?.[1] ?? '')

    const { csrf, cookie } = csrfPair()
    const ok = await app.request('http://idp.test/api/auth/reset_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ token: rawToken, new_password: 'fresh-pass-9182' }),
    })
    expect(ok.status).toBe(200)
    const replay = await app.request('http://idp.test/api/auth/reset_password', {
      method: 'POST',
      headers: { 'content-type': 'application/json', 'X-CSRF-Token': csrf, cookie },
      body: JSON.stringify({ token: rawToken, new_password: 'another-pass-1234' }),
    })
    expect(replay.status).toBe(410)
  })
})

// 二重実装にならないことの確認: token はサーバ side で SHA-256 ハッシュ化されて保存される
describe('token storage', () => {
  it('保存される token_hash は SHA-256 (生 token は持たない)', async () => {
    const { app, emailSender, passwordResetTokenStore } = await setup()
    await issueResetToken(app)
    const sentUrl = emailSender.sent[0].text
    const tokenMatch = sentUrl.match(/token=([A-Za-z0-9_-]+)/)
    const rawToken = decodeURIComponent(tokenMatch?.[1] ?? '')
    const expectedHash = createHash('sha256').update(rawToken, 'utf8').digest('hex')
    const record = await passwordResetTokenStore.consume(expectedHash, new Date())
    expect(record?.sub).toBe('user-alice')
  })
})
