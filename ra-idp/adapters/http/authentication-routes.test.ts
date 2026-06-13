/**
 * Layer 4 — Adapter Layer (HTTP: /login throttle 統合テスト)
 *
 * ログイン スロットリング (ADR-029) のシナリオを HTTP 境界で検証する:
 *   - per-account の閾値到達で 429 + Retry-After
 *   - per-IP の閾値到達で 429
 *   - 列挙対策 (存在しない username でも constant-time verify と counter 共有)
 *   - 成功時 per-account counter クリア
 *   - LoginThrottled イベントの emit
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'

import { createAuthenticationRoutes } from './authentication-routes'
import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import {
  InMemoryLoginAttemptThrottle,
  type LoginThrottleConfigs,
} from '../persistence/memory/login-attempt-throttle'
import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import type { LoginContinuation } from '../../src/authentication/ports/login-continuation'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import { UserSchema, type DomainEvent } from '../../src/spec-bindings/schemas'
import { createCsrfToken } from '../../src/shared/web-security'

const SMALL_CONFIGS: LoginThrottleConfigs = {
  account: { maxFailures: 3, windowSeconds: 60, lockoutSeconds: 120 },
  ip: { maxFailures: 5, windowSeconds: 60, lockoutSeconds: 60 },
}

async function setup(options: { trustedForwardedHops?: number; disableAlice?: boolean } = {}) {
  const userRepo = new InMemoryUserRepository()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const passwordHasher = new Argon2idPasswordHasher(8, 1)
  const throttle = new InMemoryLoginAttemptThrottle(SMALL_CONFIGS)
  const sentinelPasswordHash = await passwordHasher.hash('sentinel-test')
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)
  const verifyCalls: string[] = []

  // PasswordHasher を decorate して verify 呼び出しを記録 (列挙対策の検証用)
  const decoratedHasher = {
    hash: (p: string) => passwordHasher.hash(p),
    verify: (p: string, h: string) => {
      verifyCalls.push(h)
      return passwordHasher.verify(p, h)
    },
  }

  await userRepo.save(
    UserSchema.parse({
      sub: 'user-alice',
      tenant_id: 'default',
      preferred_username: 'alice',
      password_hash: await passwordHasher.hash('correct-password-1'),
      email_verified: true,
      mfa_enrolled: false,
      ...(options.disableAlice ? { disabled_at: '2024-02-01T00:00:00.000Z' } : {}),
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )

  // 連続ポストが認可フローの transaction を要求しないよう、form POST /login のみ
  // を使う (transaction store なしで完結する経路)。続行に失敗するレスポンスは
  // throttle の挙動とは独立なので、必要なら spy continuation で対応。
  const continuation: LoginContinuation = {
    async continueAfterLogin() {
      return new Response('continued', { status: 200 })
    },
  }

  const app = new Hono()
  app.route(
    '/',
    createAuthenticationRoutes({
      userRepo,
      passwordHasher: decoratedHasher,
      sessionManager,
      continuation,
      emit,
      loginAttemptThrottle: throttle,
      sentinelPasswordHash,
      trustedForwardedHops: options.trustedForwardedHops ?? 0,
    }),
  )

  return { app, events, verifyCalls, sentinelPasswordHash }
}

function csrfPair() {
  const csrf = createCsrfToken()
  return { csrf, cookie: `ra_idp_csrf=${csrf}` }
}

async function postLogin(
  app: Hono,
  args: {
    username: string
    password: string
    xff?: string
    csrfPair?: { csrf: string; cookie: string }
  },
): Promise<Response> {
  const { csrf, cookie } = args.csrfPair ?? csrfPair()
  const headers: Record<string, string> = {
    'content-type': 'application/x-www-form-urlencoded',
    cookie,
  }
  if (args.xff) headers['x-forwarded-for'] = args.xff
  const body = new URLSearchParams({
    request_id: 'dummy-tx',
    username: args.username,
    password: args.password,
    csrf,
  }).toString()
  return app.request('http://idp.example.com/login', {
    method: 'POST',
    headers,
    body,
  })
}

describe('login throttle — per-account', () => {
  it('しきい値到達で 429 + Retry-After + LoginThrottled イベント', async () => {
    const { app, events } = await setup()
    // 3 失敗で account ロック
    for (let i = 0; i < 3; i++) {
      const res = await postLogin(app, { username: 'alice', password: 'wrong' })
      expect(res.status).toBe(401)
    }
    const thrown = events.filter((e) => e.type === 'LoginThrottled')
    expect(thrown).toHaveLength(1)
    expect(thrown[0]).toMatchObject({ kind: 'account', retryAfterSeconds: 120 })

    // 次の試行は 429
    const blocked = await postLogin(app, { username: 'alice', password: 'wrong' })
    expect(blocked.status).toBe(429)
    expect(blocked.headers.get('retry-after')).toBe('120')
  })

  it('username は大文字小文字非依存で同じ counter に集約される', async () => {
    const { app } = await setup()
    for (let i = 0; i < 3; i++) {
      await postLogin(app, { username: i < 2 ? 'alice' : 'ALICE', password: 'wrong' })
    }
    const blocked = await postLogin(app, { username: 'Alice', password: 'wrong' })
    expect(blocked.status).toBe(429)
  })

  it('成功すると per-account counter がクリアされる', async () => {
    const { app } = await setup()
    for (let i = 0; i < 2; i++) {
      await postLogin(app, { username: 'alice', password: 'wrong' })
    }
    const ok = await postLogin(app, { username: 'alice', password: 'correct-password-1' })
    expect(ok.status).toBe(200)
    // クリア後は再び 3 回まで失敗可能
    for (let i = 0; i < 2; i++) {
      const res = await postLogin(app, { username: 'alice', password: 'wrong' })
      expect(res.status).toBe(401)
    }
  })
})

describe('login throttle — per-IP', () => {
  it('X-Forwarded-For ベースの per-IP throttle (trustedHops=1)', async () => {
    const { app, events } = await setup({ trustedForwardedHops: 1 })
    // 同一 IP で 5 失敗 → IP ロック。それぞれ別 username で account ロックを避ける。
    for (let i = 0; i < 5; i++) {
      const res = await postLogin(app, {
        username: `unknown-${i}`,
        password: 'wrong',
        xff: '203.0.113.7, 10.0.0.1',
      })
      expect(res.status).toBe(401)
    }
    const thrown = events.filter((e) => e.type === 'LoginThrottled')
    expect(thrown).toHaveLength(1)
    expect(thrown[0]).toMatchObject({ kind: 'ip', retryAfterSeconds: 60 })

    const blocked = await postLogin(app, {
      username: 'unknown-x',
      password: 'wrong',
      xff: '203.0.113.7, 10.0.0.1',
    })
    expect(blocked.status).toBe(429)
  })

  it('trustedHops=0 (デフォルト) では XFF を信頼せず per-IP は適用されない', async () => {
    const { app, events } = await setup({ trustedForwardedHops: 0 })
    for (let i = 0; i < 5; i++) {
      await postLogin(app, {
        username: `unknown-${i}`,
        password: 'wrong',
        xff: '203.0.113.7, 10.0.0.1',
      })
    }
    const thrown = events.filter((e) => e.type === 'LoginThrottled')
    expect(thrown).toHaveLength(0)
  })
})

describe('disabled user (ADR-031)', () => {
  it('正しい password でも disabled なら 401 を返し account_disabled を emit する', async () => {
    const { app, events } = await setup({ disableAlice: true })
    const res = await postLogin(app, { username: 'alice', password: 'correct-password-1' })
    expect(res.status).toBe(401)
    const failed = events.find(
      (e) =>
        e.type === 'AuthenticationFailed' &&
        (e as { reason?: string }).reason === 'account_disabled',
    )
    expect(failed).toBeDefined()
    expect(events.some((e) => e.type === 'UserAuthenticated')).toBe(false)
  })
})

describe('login throttle — ユーザー名列挙対策', () => {
  it('存在しない username でも passwordHasher.verify が呼ばれる (constant-time)', async () => {
    const { app, verifyCalls, sentinelPasswordHash } = await setup()
    await postLogin(app, { username: 'ghost', password: 'whatever' })
    expect(verifyCalls).toContain(sentinelPasswordHash)
  })

  it('存在しない username の失敗も per-account counter を消費する', async () => {
    const { app } = await setup()
    for (let i = 0; i < 3; i++) {
      await postLogin(app, { username: 'ghost', password: 'whatever' })
    }
    const blocked = await postLogin(app, { username: 'ghost', password: 'whatever' })
    expect(blocked.status).toBe(429)
  })
})
