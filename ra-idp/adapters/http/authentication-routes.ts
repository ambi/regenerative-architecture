/**
 * Layer 4 — Adapter Layer (HTTP: /login)
 *
 * Authentication component HTTP boundary. It verifies user credentials,
 * creates the browser login session, and delegates OAuth2/OIDC resumption to
 * a continuation port.
 */

import { createHash } from 'crypto'
import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { LoginContinuation } from '../../src/authentication/ports/login-continuation'
import type {
  LoginAttemptThrottle,
  LoginThrottleKind,
} from '../../src/authentication/ports/login-attempt-throttle'
import {
  SESSION_COOKIE,
  SESSION_TTL_SECONDS,
  type SessionManager,
} from '../../src/authentication/usecases/session-manager'
import type { PasswordHasher } from '../../src/authentication/ports/password-hasher'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import {
  assertCsrf,
  createCsrfToken,
  csrfCookie,
  sessionCookie,
  WebSecurityError,
} from '../../src/shared/web-security'
import { oauthErrorResponse } from './error-response'
import { extractClientIp } from './extract-client-ip'
import { renderShell } from './spa-shell'
import { noStoreJSON, transactionIdFromCookie } from './browser-transaction'

export interface AuthenticationRoutesDeps {
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  sessionManager: SessionManager
  continuation: LoginContinuation
  emit: (e: DomainEvent) => void
  /** ADR-029。passwordHasher の事前ハッシュ値（constant-time verify 用 sentinel）。 */
  loginAttemptThrottle: LoginAttemptThrottle
  sentinelPasswordHash: string
  trustedForwardedHops: number
}

export function createAuthenticationRoutes(deps: AuthenticationRoutesDeps) {
  const app = new Hono()

  // ブラウザが直接 GET /login で戻ってきた場合 (SPA back/forward 等) に
  // 同じ shell を返す。`request_id` が無い場合は SPA 側で「セッション開始」
  // をハンドリングする。
  app.get('/login', (c) => {
    const requestId =
      transactionIdFromCookie(c.req.header('Cookie')) || c.req.query('request_id') || ''
    return loginRequiredResponse(requestId, c.req.header('accept-language'))
  })

  app.post('/api/auth/login', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const body = await c.req.json().catch(() => null)
      const username = typeof body?.username === 'string' ? body.username : ''
      const password = typeof body?.password === 'string' ? body.password : ''
      const requestId = transactionIdFromCookie(c.req.header('Cookie'))
      if (!requestId) {
        return noStoreJSON(c, 401, {
          error: 'transaction_unavailable',
          message: '認可トランザクションがありません',
        })
      }
      const clientIp = extractClientIp(c.req.raw.headers, {
        trustedHops: deps.trustedForwardedHops,
      })
      const response = await completePasswordLogin(
        deps,
        requestId,
        username,
        password,
        clientIp,
        c.req.header('accept-language'),
        true,
      )
      return response
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      if (e instanceof WebSecurityError) {
        return noStoreJSON(c, 403, { error: 'csrf_failed', message: e.message })
      }
      throw e
    }
  })

  app.post('/login', async (c) => {
    try {
      const body = await c.req.parseBody()
      const requestId = String(body.request_id ?? '')
      const username = String(body.username ?? '')
      const password = String(body.password ?? '')
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))

      const clientIp = extractClientIp(c.req.raw.headers, {
        trustedHops: deps.trustedForwardedHops,
      })
      const response = await completePasswordLogin(
        deps,
        requestId,
        username,
        password,
        clientIp,
        c.req.header('accept-language'),
        false,
      )
      // CSRF Cookie はクリアしない。次のページ（consent / totp）が新しい CSRF Cookie を
      // 同じ名前でセットするため、ここで Max-Age=0 を append するとブラウザが
      // 「最後の Set-Cookie が勝つ」順序で消してしまい後続が CSRF 不一致になる。
      return response
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      if (e instanceof WebSecurityError) {
        return oauthErrorResponse(c, new OAuthError('invalid_request', e.message))
      }
      throw e
    }
  })

  return app
}

async function completePasswordLogin(
  deps: AuthenticationRoutesDeps,
  requestId: string,
  username: string,
  password: string,
  clientIp: string | null,
  acceptLanguage?: string,
  browserAPI = false,
): Promise<Response> {
  const normalizedUsername = username.toLowerCase()
  const now = new Date()

  const accountAcquire = await deps.loginAttemptThrottle.tryAcquire(
    'account',
    normalizedUsername,
    now,
  )
  if (!accountAcquire.allowed) {
    return throttledResponse(accountAcquire.retryAfterSeconds ?? 0, browserAPI)
  }
  if (clientIp) {
    const ipAcquire = await deps.loginAttemptThrottle.tryAcquire('ip', clientIp, now)
    if (!ipAcquire.allowed) {
      return throttledResponse(ipAcquire.retryAfterSeconds ?? 0, browserAPI)
    }
  }

  // ADR-029: 未存在 user でも sentinel ハッシュで verify を回し、タイミング差で
  // username の存在有無を漏らさない。
  const user = await deps.userRepo.findByUsername(username)
  const hashToVerify = user?.password_hash ?? deps.sentinelPasswordHash
  const passwordOk = await deps.passwordHasher.verify(password, hashToVerify)

  if (!user || !passwordOk) {
    await recordLoginFailure(deps, normalizedUsername, clientIp, now)
    deps.emit({
      type: 'AuthenticationFailed',
      occurredAt: now.toISOString(),
      username,
      reason: 'invalid_credentials',
    })
    return loginRequiredResponse(requestId, acceptLanguage)
  }

  // ADR-031: disabled_at が立った user は password が正しくても弾く。
  // user enumeration を避けるため UI 表面の応答は invalid_credentials と同じだが、
  // 監査ログでは account_disabled として区別する。
  if (user.disabled_at) {
    deps.emit({
      type: 'AuthenticationFailed',
      occurredAt: now.toISOString(),
      username,
      reason: 'account_disabled',
    })
    return loginRequiredResponse(requestId, acceptLanguage)
  }

  // 成功: per-account のみクリア (ADR-029)。per-IP は時間で自然失効させる。
  await deps.loginAttemptThrottle.recordSuccess('account', normalizedUsername)

  const needsSecondFactor = user.mfa_enrolled
  const context = await deps.sessionManager.create(user.tenant_id, user.sub, ['pwd'], now, {
    authenticationPending: needsSecondFactor,
  })
  deps.emit({
    type: 'UserAuthenticated',
    occurredAt: now.toISOString(),
    sub: user.sub,
    amr: ['pwd'],
  })

  const response = needsSecondFactor
    ? browserAPI
      ? browserFlowJSON({ next: '/totp' })
      : new Response(null, { status: 303, headers: { location: '/totp' } })
    : await deps.continuation.continueAfterLogin(requestId, context, {
        promptLoginSatisfied: true,
        acceptLanguage,
      })
  if (context.session_id) {
    response.headers.append(
      'set-cookie',
      sessionCookie(SESSION_COOKIE, context.session_id, SESSION_TTL_SECONDS),
    )
  }
  return browserAPI ? responseToBrowserFlow(response) : response
}

function browserFlowJSON(body: { next?: string; redirect_to?: string }): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
}

function responseToBrowserFlow(response: Response): Response {
  const location = response.headers.get('location')
  if (location) {
    return copySetCookie(response, browserFlowJSON({ redirect_to: location }))
  }
  if (response.headers.get('content-type')?.includes('text/html')) {
    return copySetCookie(response, browserFlowJSON({ next: '/consent' }))
  }
  return response
}

function copySetCookie(from: Response, to: Response): Response {
  for (const setCookie of from.headers.getSetCookie()) {
    to.headers.append('set-cookie', setCookie)
  }
  return to
}

async function recordLoginFailure(
  deps: AuthenticationRoutesDeps,
  normalizedUsername: string,
  clientIp: string | null,
  now: Date,
): Promise<void> {
  const accountResult = await deps.loginAttemptThrottle.recordFailure(
    'account',
    normalizedUsername,
    now,
  )
  if (accountResult.locked) {
    deps.emit(
      buildThrottledEvent('account', normalizedUsername, accountResult.retryAfterSeconds, now),
    )
  }
  if (clientIp) {
    const ipResult = await deps.loginAttemptThrottle.recordFailure('ip', clientIp, now)
    if (ipResult.locked) {
      deps.emit(buildThrottledEvent('ip', clientIp, ipResult.retryAfterSeconds, now))
    }
  }
}

function buildThrottledEvent(
  kind: LoginThrottleKind,
  key: string,
  retryAfterSeconds: number | undefined,
  now: Date,
): DomainEvent {
  return {
    type: 'LoginThrottled',
    occurredAt: now.toISOString(),
    kind,
    keyHash: createHash('sha256').update(key, 'utf8').digest('hex'),
    retryAfterSeconds: retryAfterSeconds ?? 0,
  }
}

function throttledResponse(retryAfterSeconds: number, browserAPI: boolean): Response {
  const headers: Record<string, string> = {
    'retry-after': String(retryAfterSeconds),
    'cache-control': 'no-store',
  }
  if (browserAPI) {
    return new Response(
      JSON.stringify({
        error: 'too_many_requests',
        retry_after_seconds: retryAfterSeconds,
      }),
      {
        status: 429,
        headers: { ...headers, 'content-type': 'application/json; charset=UTF-8' },
      },
    )
  }
  return new Response('Too Many Requests\n', {
    status: 429,
    headers: { ...headers, 'content-type': 'text/plain; charset=UTF-8' },
  })
}

export function loginRequiredResponse(requestId: string, acceptLanguage?: string): Response {
  const csrf = createCsrfToken()
  // SPA shell + 隠しフォーム (no-JS / テスト fallback)。
  // POST /api/auth/login は X-CSRF-Token、no-JS/form fallback の POST /login は
  // body の csrf を `csrfCookie` の値と二重提出パターンで照合する。
  const html = renderShell({
    page: 'login',
    title: 'サインイン',
    meta: { 'request-id': requestId, csrf },
    fallbackForm: {
      action: '/login',
      fields: { request_id: requestId, csrf },
    },
    acceptLanguage,
  })
  return new Response(html, {
    status: 401,
    headers: {
      'content-type': 'text/html; charset=UTF-8',
      'set-cookie': csrfCookie(csrf),
    },
  })
}
