/**
 * Layer 4 — Adapter Layer（HTTP: /authorize / /end_session）
 *
 * OIDC セッション系シナリオを HTTP 入力境界で検証する。
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { createHash } from 'crypto'

import { createAuthenticationRoutes } from './authentication-routes'
import { createAuthorizationLoginContinuation, createAuthorizeRoutes } from './authorize-routes'
import {
  InMemoryAuthorizationCodeStore,
  InMemoryAuthorizationRequestStore,
  InMemoryPARStore,
} from '../persistence/memory/authorization-store'
import { InMemoryClientRepository } from '../persistence/memory/client-repo'
import { InMemoryConsentRepository } from '../persistence/memory/consent-repo'
import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import {
  InMemoryLoginAttemptThrottle,
  type LoginThrottleConfigs,
} from '../persistence/memory/login-attempt-throttle'
import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import { DemoHeaderResolver } from '../../src/authentication/usecases/demo-header-resolver'
import type { AuthenticationContextResolver } from '../../src/authentication/domain/authentication-context'
import {
  ClientSchema,
  UserSchema,
  type Client,
  type DomainEvent,
} from '../../src/spec-bindings/schemas'

// テストでは throttle が偶発的に発火しないよう、しきい値を十分大きく取る。
const TEST_LOGIN_THROTTLE_CONFIGS: LoginThrottleConfigs = {
  account: { maxFailures: 1000, windowSeconds: 60, lockoutSeconds: 60 },
  ip: { maxFailures: 1000, windowSeconds: 60, lockoutSeconds: 60 },
}
// ADR-029 の sentinel ハッシュ。テスト用に固定の弱いパラメータで 1 度だけ計算する。
const testSentinelHash = await new Argon2idPasswordHasher(8, 1).hash('sentinel-test-secret')

function makeClient(overrides: Partial<Client> = {}): Client {
  return ClientSchema.parse({
    client_id: 'web-app',
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile offline_access',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
    ...overrides,
  })
}

async function setup() {
  const clientRepo = new InMemoryClientRepository()
  const userRepo = new InMemoryUserRepository()
  const consentRepo = new InMemoryConsentRepository()
  const requestStore = new InMemoryAuthorizationRequestStore()
  const codeStore = new InMemoryAuthorizationCodeStore()
  const parStore = new InMemoryPARStore()
  const sessionStore = new InMemorySessionStore()
  const sessionManager = new LoginSessionManager(sessionStore)
  const passwordHasher = new Argon2idPasswordHasher()
  const demoHeaderResolver = new DemoHeaderResolver(userRepo)
  const authenticationContextResolver: AuthenticationContextResolver = {
    async resolve(headers) {
      return (await sessionManager.resolve(headers)) ?? (await demoHeaderResolver.resolve(headers))
    },
  }
  const events: DomainEvent[] = []
  const client = makeClient()

  await clientRepo.save(client)
  await userRepo.save(
    UserSchema.parse({
      sub: 'user_alice',
      preferred_username: 'alice',
      password_hash: await passwordHasher.hash('pw'),
      email: 'alice@example.com',
      email_verified: true,
      mfa_enrolled: false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }),
  )
  await consentRepo.save({
    sub: 'user_alice',
    client_id: client.client_id,
    scopes: ['openid', 'profile'],
    granted_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 86400_000).toISOString(),
  })

  const app = new Hono()
  const emit = (e: DomainEvent) => events.push(e)
  const authorizeDeps = {
    issuer: 'http://idp.example.com',
    clientRepo,
    consentRepo,
    userRepo,
    requestStore,
    codeStore,
    parStore,
    authenticationContextResolver,
    sessionManager,
    emit,
  }
  app.route('/', createAuthorizeRoutes(authorizeDeps))
  app.route(
    '/',
    createAuthenticationRoutes({
      userRepo,
      passwordHasher,
      sessionManager,
      continuation: createAuthorizationLoginContinuation(authorizeDeps),
      emit,
      loginAttemptThrottle: new InMemoryLoginAttemptThrottle(TEST_LOGIN_THROTTLE_CONFIGS),
      sentinelPasswordHash: testSentinelHash,
      trustedForwardedHops: 0,
    }),
  )

  return { app, events, sessionStore }
}

function authorizeUrl(extra: Record<string, string> = {}): string {
  const url = new URL('http://idp.example.com/authorize')
  url.searchParams.set('client_id', 'web-app')
  url.searchParams.set('redirect_uri', 'https://app.example.com/cb')
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'openid profile')
  url.searchParams.set('code_challenge', 'challenge')
  url.searchParams.set('code_challenge_method', 'S256')
  for (const [k, v] of Object.entries(extra)) url.searchParams.set(k, v)
  return url.toString()
}

describe('authorize routes — OIDC session prompts', () => {
  it('prompt=none は未認証時に access_denied を返す', async () => {
    const { app } = await setup()

    const res = await app.request(authorizeUrl({ prompt: 'none' }))

    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'access_denied' })
  })

  it('X-User-Auth-Time が max_age を超えている場合はログインを要求する', async () => {
    const { app } = await setup()

    const res = await app.request(authorizeUrl({ max_age: '60' }), {
      headers: {
        'X-User-Sub': 'user_alice',
        'X-User-Auth-Time': String(Math.floor(Date.now() / 1000) - 120),
      },
    })

    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/login')
    const login = await app.request('http://idp.example.com/login', {
      headers: { cookie: res.headers.get('set-cookie') ?? '' },
    })
    expect(login.status).toBe(401)
    const body = await login.text()
    // SPA shell + 隠しフォーム fallback。サインインの hidden input を含むこと。
    expect(body).toContain('name="ra-idp:page" content="login"')
    expect(body).toContain('name="csrf"')
  })

  it('ログインフォームの成功後はセッション Cookie を発行し認可コードへリダイレクトする', async () => {
    const { app, events } = await setup()

    const loginPage = await getLoginPage(app, authorizeUrl())
    expect(loginPage.status).toBe(401)
    const loginHtml = await loginPage.text()
    const requestId = extractInput(loginHtml, 'request_id')
    const csrf = extractInput(loginHtml, 'csrf')
    const csrfCookie = loginPage.headers.get('set-cookie') ?? ''

    const res = await app.request('http://idp.example.com/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: csrfCookie,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf,
        username: 'alice',
        password: 'pw',
      }).toString(),
    })

    expect(res.status).toBe(302)
    const loc = res.headers.get('location') ?? ''
    expect(loc).toStartWith('https://app.example.com/cb?code=')
    // RFC 9207: 認可レスポンスに iss を含める
    expect(new URL(loc).searchParams.get('iss')).toBe('http://idp.example.com')
    expect(res.headers.get('set-cookie')).toContain('ra_idp_session=')
    expect(events.some((e) => e.type === 'UserAuthenticated')).toBe(true)
    expect(events.some((e) => e.type === 'AuthorizationCodeIssued')).toBe(true)
  })

  it('セッション Cookie から AuthenticationContext が復元される', async () => {
    const { app } = await setup()

    const loginPage = await getLoginPage(app, authorizeUrl())
    const loginHtml = await loginPage.text()
    const requestId = extractInput(loginHtml, 'request_id')
    const csrf = extractInput(loginHtml, 'csrf')
    const csrfCookie = loginPage.headers.get('set-cookie') ?? ''

    const login = await app.request('http://idp.example.com/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: csrfCookie,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf,
        username: 'alice',
        password: 'pw',
      }).toString(),
    })
    const sessionCookie = login.headers.get('set-cookie') ?? ''

    const res = await app.request(authorizeUrl({ state: 'from-cookie' }), {
      headers: { cookie: sessionCookie },
    })

    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toContain('state=from-cookie')
  })

  it('ログインフォームは CSRF 不一致を拒否する', async () => {
    const { app } = await setup()
    const loginPage = await getLoginPage(app, authorizeUrl())
    const loginHtml = await loginPage.text()
    const requestId = extractInput(loginHtml, 'request_id')

    const res = await app.request('http://idp.example.com/login', {
      method: 'POST',
      headers: { 'content-type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({
        request_id: requestId,
        csrf: 'bad',
        username: 'alice',
        password: 'pw',
      }).toString(),
    })

    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'invalid_request' })
  })

  it('ログイン後のコンセント画面で発行された CSRF Cookie が消されずに POST /consent が通る', async () => {
    const { app } = await setup()

    // 既存 consent (openid + profile) に含まれない offline_access を要求し、consent UI を強制する
    const loginPage = await getLoginPage(
      app,
      authorizeUrl({ scope: 'openid profile offline_access' }),
    )
    const loginHtml = await loginPage.text()
    const loginRequestId = extractInput(loginHtml, 'request_id')
    const loginCsrf = extractInput(loginHtml, 'csrf')
    const loginCsrfCookie = loginPage.headers.get('set-cookie') ?? ''

    const consentPage = await app.request('http://idp.example.com/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: loginCsrfCookie,
      },
      body: new URLSearchParams({
        request_id: loginRequestId,
        csrf: loginCsrf,
        username: 'alice',
        password: 'pw',
      }).toString(),
    })

    expect(consentPage.status).toBe(200)
    const consentHtml = await consentPage.text()
    const consentRequestId = extractInput(consentHtml, 'request_id')
    const consentCsrf = extractInput(consentHtml, 'csrf')
    const consentSetCookies = consentPage.headers.getSetCookie()
    const lastCsrfDirective =
      consentSetCookies.filter((c) => c.startsWith('ra_idp_csrf=')).at(-1) ?? ''
    expect(lastCsrfDirective).not.toContain('Max-Age=0')

    const res = await app.request('http://idp.example.com/consent', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `ra_idp_csrf=${consentCsrf}`,
      },
      body: new URLSearchParams({
        request_id: consentRequestId,
        csrf: consentCsrf,
        action: 'allow',
      }).toString(),
    })

    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toStartWith('https://app.example.com/cb?code=')
  })

  it('ブラウザ API の同意許可は再送されても code を二重発行しない', async () => {
    const { app } = await setup()
    const start = await app.request(authorizeUrl({ scope: 'openid profile offline_access' }))
    expect(start.status).toBe(303)
    expect(start.headers.get('location')).toBe('/login')
    const transactionCookie = cookiePairFrom(start.headers, 'ra_idp_transaction')

    const loginTransaction = await app.request('http://idp.example.com/api/auth/transaction', {
      headers: { cookie: transactionCookie },
    })
    expect(loginTransaction.status).toBe(200)
    const loginCsrfCookie = cookiePairFrom(loginTransaction.headers, 'ra_idp_csrf')
    const loginTransactionBody = (await loginTransaction.json()) as {
      kind: string
      csrf_token: string
    }
    expect(loginTransactionBody).toMatchObject({ kind: 'login' })

    const login = await app.request('http://idp.example.com/api/auth/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'X-CSRF-Token': loginTransactionBody.csrf_token,
        cookie: `${transactionCookie}; ${loginCsrfCookie}`,
      },
      body: JSON.stringify({ username: 'alice', password: 'pw' }),
    })
    expect(login.status).toBe(200)
    expect(await login.json()).toMatchObject({ next: '/consent' })
    const sessionCookie = cookiePairFrom(login.headers, 'ra_idp_session')

    const consentTransaction = await app.request('http://idp.example.com/api/auth/transaction', {
      headers: { cookie: `${transactionCookie}; ${sessionCookie}` },
    })
    expect(consentTransaction.status).toBe(200)
    const consentCsrfCookie = cookiePairFrom(consentTransaction.headers, 'ra_idp_csrf')
    const consentTransactionBody = (await consentTransaction.json()) as {
      kind: string
      csrf_token: string
    }
    expect(consentTransactionBody).toMatchObject({ kind: 'consent' })

    const headers = {
      'content-type': 'application/json',
      'X-CSRF-Token': consentTransactionBody.csrf_token,
      cookie: `${transactionCookie}; ${sessionCookie}; ${consentCsrfCookie}`,
    }
    const first = await app.request('http://idp.example.com/api/auth/consent', {
      method: 'POST',
      headers,
      body: JSON.stringify({ action: 'allow' }),
    })
    const second = await app.request('http://idp.example.com/api/auth/consent', {
      method: 'POST',
      headers,
      body: JSON.stringify({ action: 'allow' }),
    })

    expect(first.status).toBe(200)
    expect(second.status).toBe(200)
    const firstLocation = ((await first.json()) as { redirect_to: string }).redirect_to
    const secondLocation = ((await second.json()) as { redirect_to: string }).redirect_to
    expect(firstLocation).toStartWith('https://app.example.com/cb?code=')
    expect(secondLocation).toBe(firstLocation)
  })
})

describe('authorize routes — RP-Initiated Logout', () => {
  it('end_session は登録済み post_logout_redirect_uri に state 付きでリダイレクトする', async () => {
    const { app } = await setup()

    const res = await app.request(
      'http://idp.example.com/end_session?client_id=web-app&post_logout_redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&state=s1',
    )

    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toBe('https://app.example.com/cb?state=s1')
  })

  it('end_session は未登録 post_logout_redirect_uri を拒否する', async () => {
    const { app } = await setup()

    const res = await app.request(
      'http://idp.example.com/end_session?client_id=web-app&post_logout_redirect_uri=https%3A%2F%2Fevil.example.com%2Fcb',
    )

    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'invalid_request' })
  })

  it('end_session は SessionManager.revoke() にセッション削除を委譲する', async () => {
    const { app, sessionStore } = await setup()
    await sessionStore.save({
      id: 'sid-1',
      sub: 'user_alice',
      auth_time: Math.floor(Date.now() / 1000),
      amr: ['pwd'],
      acr: 'urn:ra-idp:acr:pwd',
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
    })

    const res = await app.request('http://idp.example.com/end_session', {
      headers: { cookie: 'ra_idp_session=sid-1' },
    })

    expect(res.status).toBe(200)
    expect(await sessionStore.find('sid-1')).toBeNull()
    expect(res.headers.get('set-cookie')).toContain('ra_idp_session=;')
  })
})

async function getLoginPage(app: Hono, authorize: string): Promise<Response> {
  const redirect = await app.request(authorize)
  expect(redirect.status).toBe(303)
  expect(redirect.headers.get('location')).toBe('/login')
  return await app.request('http://idp.example.com/login', {
    headers: { cookie: redirect.headers.get('set-cookie') ?? '' },
  })
}

function extractInput(html: string, name: string): string {
  const match = html.match(new RegExp(`name="${name}" value="([^"]+)"`))
  if (!match) throw new Error(`input ${name} not found`)
  return match[1]
}

function cookiePair(setCookie: string): string {
  return setCookie.split(';')[0] ?? ''
}

function cookiePairFrom(headers: Headers, name: string): string {
  const setCookie =
    headers.getSetCookie().find((cookie) => cookie.startsWith(`${name}=`)) ??
    headers.get('set-cookie') ??
    ''
  return cookiePair(setCookie)
}
