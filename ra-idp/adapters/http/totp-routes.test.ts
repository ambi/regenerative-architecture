/**
 * Layer 4 — Adapter Layer (HTTP: /totp 統合シナリオ)
 *
 * パスワード成功後の TOTP challenge → 認可コード発行までを HTTP 境界で検証する。
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { createHash } from 'crypto'

import { createAuthenticationRoutes } from './authentication-routes'
import { createAuthorizationLoginContinuation, createAuthorizeRoutes } from './authorize-routes'
import { createTotpRoutes } from './totp-routes'
import {
  InMemoryAuthorizationCodeStore,
  InMemoryAuthorizationRequestStore,
  InMemoryPARStore,
} from '../persistence/memory/authorization-store'
import { InMemoryClientRepository } from '../persistence/memory/client-repo'
import { InMemoryConsentRepository } from '../persistence/memory/consent-repo'
import { InMemoryMfaFactorRepository } from '../persistence/memory/mfa-factor-repo'
import { InMemorySessionStore } from '../persistence/memory/session-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { Argon2idPasswordHasher } from '../crypto/argon2id-password-hasher'
import { LoginSessionManager } from '../../src/authentication/usecases/session-manager'
import { generateTotp } from '../../src/authentication/usecases/totp'
import { DemoHeaderResolver } from '../../src/authentication/usecases/demo-header-resolver'
import type { AuthenticationContextResolver } from '../../src/authentication/domain/authentication-context'
import {
  ClientSchema,
  UserSchema,
  type Client,
  type DomainEvent,
} from '../../src/spec-bindings/schemas'

const TOTP_SECRET = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'

function makeClient(overrides: Partial<Client> = {}): Client {
  return ClientSchema.parse({
    client_id: 'web-app',
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
    ...overrides,
  })
}

async function setup(options: { mfaEnrolled?: boolean; prefillConsent?: boolean } = {}) {
  const mfaEnrolled = options.mfaEnrolled ?? true
  const prefillConsent = options.prefillConsent ?? true
  const clientRepo = new InMemoryClientRepository()
  const userRepo = new InMemoryUserRepository()
  const mfaFactorRepo = new InMemoryMfaFactorRepository()
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
      mfa_enrolled: mfaEnrolled,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }),
  )
  if (mfaEnrolled) {
    await mfaFactorRepo.save({
      sub: 'user_alice',
      type: 'totp',
      secret: TOTP_SECRET,
      created_at: new Date().toISOString(),
    })
  }
  if (prefillConsent) {
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })
  }

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
  const app = new Hono()
  app.route('/', createAuthorizeRoutes(authorizeDeps))
  app.route(
    '/',
    createAuthenticationRoutes({
      userRepo,
      passwordHasher,
      sessionManager,
      continuation: createAuthorizationLoginContinuation(authorizeDeps),
      emit,
    }),
  )
  app.route(
    '/',
    createTotpRoutes({
      sessionManager,
      mfaFactorRepo,
      continuation: createAuthorizationLoginContinuation(authorizeDeps),
      emit,
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

function extractInput(html: string, name: string): string {
  const m = html.match(new RegExp(`name="${name}" value="([^"]+)"`))
  if (!m) throw new Error(`hidden input ${name} not found`)
  return m[1]
}

async function passwordLogin(
  app: Hono,
  extra: Record<string, string> = {},
): Promise<{
  requestId: string
  sessionCookie: string
  totpCsrf: string
  totpCsrfCookie: string
}> {
  const start = await app.request(authorizeUrl(extra))
  expect(start.status).toBe(303)
  expect(start.headers.get('location')).toBe('/login')
  const transactionCookie = start.headers.get('set-cookie') ?? ''
  const loginPage = await app.request('http://idp.example.com/login', {
    headers: { cookie: transactionCookie },
  })
  const loginHtml = await loginPage.text()
  const requestId = extractInput(loginHtml, 'request_id')
  const csrf = extractInput(loginHtml, 'csrf')
  const csrfCookie = loginPage.headers.get('set-cookie') ?? ''
  const res = await app.request('http://idp.example.com/login', {
    method: 'POST',
    headers: {
      'content-type': 'application/x-www-form-urlencoded',
      cookie: `${transactionCookie}; ${csrfCookie}`,
    },
    body: new URLSearchParams({
      request_id: requestId,
      csrf,
      username: 'alice',
      password: 'pw',
    }).toString(),
  })
  expect(res.status).toBe(303)
  expect(res.headers.get('location')).toBe('/totp')
  const setCookieRaw = res.headers.getSetCookie?.() ?? []
  const cookies = setCookieRaw.length > 0 ? setCookieRaw : [res.headers.get('set-cookie') ?? '']
  const sessionCookie = cookies.find((h) => h.startsWith('ra_idp_session=')) ?? ''
  const totpPage = await app.request('http://idp.example.com/totp', {
    headers: { cookie: `${transactionCookie}; ${sessionCookie}` },
  })
  const totpCsrfCookie = totpPage.headers.get('set-cookie') ?? ''
  const totpCsrf = extractInput(await totpPage.text(), 'csrf')
  return {
    requestId,
    sessionCookie: `${transactionCookie}; ${sessionCookie}`,
    totpCsrf,
    totpCsrfCookie,
  }
}

describe('totp routes — login flow integration', () => {
  it('mfa_enrolled なユーザは password 成功後 /totp にリダイレクトされる', async () => {
    const { app } = await setup()
    const start = await app.request(authorizeUrl())
    expect(start.status).toBe(303)
    expect(start.headers.get('location')).toBe('/login')
    const transactionCookie = start.headers.get('set-cookie') ?? ''
    const loginPage = await app.request('http://idp.example.com/login', {
      headers: { cookie: transactionCookie },
    })
    const loginHtml = await loginPage.text()
    const requestId = extractInput(loginHtml, 'request_id')
    const csrf = extractInput(loginHtml, 'csrf')
    const csrfCookie = loginPage.headers.get('set-cookie') ?? ''

    const res = await app.request('http://idp.example.com/login', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `${transactionCookie}; ${csrfCookie}`,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf,
        username: 'alice',
        password: 'pw',
      }).toString(),
    })

    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/totp')
    const setCookies = res.headers.getSetCookie?.() ?? []
    expect(setCookies.some((c) => c.startsWith('ra_idp_session='))).toBe(true)
  })

  it('authentication_pending セッションで /authorize に来ても totp shell が返る', async () => {
    const { app } = await setup()
    const { sessionCookie } = await passwordLogin(app)
    const res = await app.request(authorizeUrl(), { headers: { cookie: sessionCookie } })
    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/totp')
    const html = await (
      await app.request('http://idp.example.com/totp', {
        headers: { cookie: `${sessionCookie}; ${res.headers.get('set-cookie') ?? ''}` },
      })
    ).text()
    expect(html).toContain('name="ra-idp:page" content="totp"')
  })

  it('GET /totp 直接アクセスも shell を返す (フォールバック)', async () => {
    const { app } = await setup()
    const { requestId, sessionCookie } = await passwordLogin(app)
    const res = await app.request(`http://idp.example.com/totp?request_id=${requestId}`, {
      headers: { cookie: sessionCookie },
    })
    expect(res.status).toBe(200)
    const html = await res.text()
    expect(html).toContain('name="ra-idp:page" content="totp"')
    expect(html).toContain('name="request_id"')
    expect(html).toContain('name="csrf"')
  })

  it('consent 未済シナリオで POST /totp 成功後の再評価が二重遷移エラーにならない', async () => {
    // no-JS/form fallback のデフォルトシナリオ: 既存 consent が無いので POST /totp は
    // consent shell (200) を返す。completeAuthenticationUseCase が冪等でないと
    // 「consent_pending に authenticate_user で進めようとしてエラー」になる。
    const { app } = await setup({ prefillConsent: false })
    const { requestId, sessionCookie, totpCsrf, totpCsrfCookie } = await passwordLogin(app)
    const totpCode = generateTotp(TOTP_SECRET, Math.floor(Date.now() / 1000))

    const first = await app.request('http://idp.example.com/totp', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `${sessionCookie}; ${totpCsrfCookie}`,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf: totpCsrf,
        code: totpCode,
      }).toString(),
    })
    expect(first.status).toBe(200)
    expect(await first.text()).toContain('name="ra-idp:page" content="consent"')

    const second = await app.request(authorizeUrl(), {
      headers: { cookie: sessionCookie },
    })
    expect(second.status).toBe(200)
    expect(await second.text()).toContain('name="ra-idp:page" content="consent"')
  })

  it.skip('GET /totp は factor 完了後の reload で OAuth2 continuation に進める (認可コード発行)', async () => {
    const { app, sessionStore } = await setup()
    const { requestId, sessionCookie } = await passwordLogin(app)
    // factor 完了状態を直接シミュレート (POST /totp 経由ではなく)
    const sessionId = sessionCookie.match(/ra_idp_session=([^;]+)/)?.[1] ?? ''
    const session = await sessionStore.find(sessionId)
    await sessionStore.save({
      ...session!,
      amr: ['pwd', 'otp'],
      acr: 'urn:ra-idp:acr:mfa',
      authentication_pending: false,
    })

    const res = await app.request(`http://idp.example.com/totp?request_id=${requestId}`, {
      headers: { cookie: sessionCookie },
    })
    expect(res.status).toBe(302)
    expect(res.headers.get('location') ?? '').toStartWith('https://app.example.com/cb?code=')
  })

  it('正しい TOTP コードで認可コードまで到達し amr=[pwd,otp] が記録される', async () => {
    const { app, events, sessionStore } = await setup()
    const { requestId, sessionCookie, totpCsrf, totpCsrfCookie } = await passwordLogin(app)

    const code = generateTotp(TOTP_SECRET, Math.floor(Date.now() / 1000))
    const res = await app.request('http://idp.example.com/totp', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `${sessionCookie}; ${totpCsrfCookie}`,
      },
      body: new URLSearchParams({ request_id: requestId, csrf: totpCsrf, code }).toString(),
    })

    expect(res.status).toBe(302)
    const loc = res.headers.get('location') ?? ''
    expect(loc).toStartWith('https://app.example.com/cb?code=')

    const sessionId = sessionCookie.match(/ra_idp_session=([^;]+)/)?.[1] ?? ''
    const session = await sessionStore.find(sessionId)
    expect(session?.amr).toEqual(['pwd', 'otp'])
    expect(session?.acr).toBe('urn:ra-idp:acr:mfa')
    expect(session?.authentication_pending).toBe(false)

    expect(events.filter((e) => e.type === 'UserAuthenticated').length).toBeGreaterThanOrEqual(2)
  })

  it('誤った TOTP コードは 401 で challenge を再表示し AuthenticationFailed を emit する', async () => {
    const { app, events } = await setup()
    const { requestId, sessionCookie, totpCsrf, totpCsrfCookie } = await passwordLogin(app)

    const res = await app.request('http://idp.example.com/totp', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `${sessionCookie}; ${totpCsrfCookie}`,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf: totpCsrf,
        code: '000000',
      }).toString(),
    })
    expect(res.status).toBe(401)
    const html = await res.text()
    expect(html).toContain('name="ra-idp:totp-invalid" content="1"')
    expect(events.some((e) => e.type === 'AuthenticationFailed')).toBe(true)
  })

  it('CSRF 不一致は invalid_request', async () => {
    const { app } = await setup()
    const { requestId, sessionCookie } = await passwordLogin(app)

    const res = await app.request('http://idp.example.com/totp', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: sessionCookie,
      },
      body: new URLSearchParams({
        request_id: requestId,
        csrf: 'bad',
        code: '000000',
      }).toString(),
    })
    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'invalid_request' })
  })
})

describe('totp routes — step-up reauth via acr_values', () => {
  /**
   * pwd だけの完了セッションを持つ状態で acr_values=mfa を要求して /authorize に来ると、
   * ログイン画面ではなく直接 /totp challenge へ誘導される。
   */
  async function pwdOnlyLoggedIn(): Promise<{
    app: Hono
    sessionStore: InMemorySessionStore
    sessionCookie: string
  }> {
    // mfa_enrolled=true の alice で setup するが、まず enrollment 経由ではなく
    // 「password のみ通った完了済みセッション」を直接 sessionStore に注入する形で
    // 状況を作る (= 既に何らかの方法で pwd 段階を完了し、その後新しい authorize
    // が acr_values=mfa を要求するケース)。
    const ctx = await setup()
    const session = {
      id: 'session-pwd-only',
      sub: 'user_alice',
      auth_time: Math.floor(Date.now() / 1000),
      amr: ['pwd'],
      acr: 'urn:ra-idp:acr:pwd',
      authentication_pending: false,
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
    }
    await ctx.sessionStore.save(session)
    return {
      app: ctx.app,
      sessionStore: ctx.sessionStore,
      sessionCookie: `ra_idp_session=${session.id}`,
    }
  }

  it('acr_values=mfa を要求する /authorize は totp shell を返す (mfa_enrolled かつ pwd のみ)', async () => {
    const { app, sessionCookie } = await pwdOnlyLoggedIn()
    const res = await app.request(authorizeUrl({ acr_values: 'urn:ra-idp:acr:mfa' }), {
      headers: { cookie: sessionCookie },
    })
    expect(res.status).toBe(303)
    expect(res.headers.get('location')).toBe('/totp')
    const html = await (
      await app.request('http://idp.example.com/totp', {
        headers: { cookie: `${sessionCookie}; ${res.headers.get('set-cookie') ?? ''}` },
      })
    ).text()
    expect(html).toContain('name="ra-idp:page" content="totp"')
  })

  it('step-up で /totp に成功すると acr=mfa に昇格して認可コードが発行される', async () => {
    const { app, sessionStore, sessionCookie } = await pwdOnlyLoggedIn()
    const challenge = await app.request(
      authorizeUrl({ acr_values: 'urn:ra-idp:acr:mfa', state: 'stepup' }),
      { headers: { cookie: sessionCookie } },
    )
    expect(challenge.status).toBe(303)
    expect(challenge.headers.get('location')).toBe('/totp')
    const transactionCookie = challenge.headers.get('set-cookie') ?? ''
    const totpPage = await app.request('http://idp.example.com/totp', {
      headers: { cookie: `${sessionCookie}; ${transactionCookie}` },
    })
    const html = await totpPage.text()
    const requestId = extractInput(html, 'request_id')
    const csrf = extractInput(html, 'csrf')
    const csrfCookieHeader = totpPage.headers.get('set-cookie') ?? ''

    const code = generateTotp(TOTP_SECRET, Math.floor(Date.now() / 1000))
    const res = await app.request('http://idp.example.com/totp', {
      method: 'POST',
      headers: {
        'content-type': 'application/x-www-form-urlencoded',
        cookie: `${sessionCookie}; ${transactionCookie}; ${csrfCookieHeader}`,
      },
      body: new URLSearchParams({ request_id: requestId, csrf, code }).toString(),
    })
    expect(res.status).toBe(302)
    const callbackUrl = res.headers.get('location') ?? ''
    expect(callbackUrl).toStartWith('https://app.example.com/cb?code=')
    expect(new URL(callbackUrl).searchParams.get('state')).toBe('stepup')

    const updated = await sessionStore.find('session-pwd-only')
    expect(updated?.amr).toEqual(['pwd', 'otp'])
    expect(updated?.acr).toBe('urn:ra-idp:acr:mfa')
  })

  it('mfa_enrolled でないユーザに acr_values=mfa を要求すると access_denied', async () => {
    const { app, sessionStore } = await setup({ mfaEnrolled: false })
    await sessionStore.save({
      id: 'session-no-mfa',
      sub: 'user_alice',
      auth_time: Math.floor(Date.now() / 1000),
      amr: ['pwd'],
      acr: 'urn:ra-idp:acr:pwd',
      authentication_pending: false,
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
    })

    const res = await app.request(authorizeUrl({ acr_values: 'urn:ra-idp:acr:mfa' }), {
      headers: { cookie: 'ra_idp_session=session-no-mfa' },
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error?: string; error_description?: string }
    expect(body.error).toBe('access_denied')
    expect(body.error_description).toContain('factor')
  })
})
