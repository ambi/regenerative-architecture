/**
 * Layer 4 — Adapter Layer (HTTP: /login)
 *
 * Authentication component HTTP boundary. It verifies user credentials,
 * creates the browser login session, and delegates OAuth2/OIDC resumption to
 * a continuation port.
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { LoginContinuation } from '../../src/authentication/ports/login-continuation'
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
import { renderShell } from './spa-shell'

export interface AuthenticationRoutesDeps {
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  sessionManager: SessionManager
  continuation: LoginContinuation
  emit: (e: DomainEvent) => void
}

export function createAuthenticationRoutes(deps: AuthenticationRoutesDeps) {
  const app = new Hono()

  // ブラウザが直接 GET /login で戻ってきた場合 (SPA back/forward 等) に
  // 同じ shell を返す。`request_id` が無い場合は SPA 側で「セッション開始」
  // をハンドリングする。
  app.get('/login', (c) => {
    const requestId = c.req.query('request_id') ?? ''
    return loginRequiredResponse(requestId, c.req.header('accept-language'))
  })

  app.post('/login', async (c) => {
    try {
      const body = await c.req.parseBody()
      const requestId = String(body.request_id ?? '')
      const username = String(body.username ?? '')
      const password = String(body.password ?? '')
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))

      const user = await deps.userRepo.findByUsername(username)
      if (!user || !(await deps.passwordHasher.verify(password, user.password_hash))) {
        deps.emit({
          type: 'AuthenticationFailed',
          occurredAt: new Date().toISOString(),
          username,
          reason: 'invalid_credentials',
        })
        return loginRequiredResponse(requestId, c.req.header('accept-language'))
      }

      const now = new Date()
      const context = await deps.sessionManager.create(user.sub, ['pwd'], now)
      deps.emit({
        type: 'UserAuthenticated',
        occurredAt: now.toISOString(),
        sub: user.sub,
        amr: ['pwd'],
      })

      const response = await deps.continuation.continueAfterLogin(requestId, context, {
        promptLoginSatisfied: true,
        acceptLanguage: c.req.header('accept-language'),
      })
      if (context.session_id) {
        response.headers.append(
          'set-cookie',
          sessionCookie(SESSION_COOKIE, context.session_id, SESSION_TTL_SECONDS),
        )
      }
      // CSRF Cookie はクリアしない。次のページ（consent）が新しい CSRF Cookie を
      // 同じ名前でセットするため、ここで Max-Age=0 を append するとブラウザが
      // 「最後の Set-Cookie が勝つ」順序で消してしまい /consent が CSRF 不一致になる。
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

export function loginRequiredResponse(requestId: string, acceptLanguage?: string): Response {
  const csrf = createCsrfToken()
  // SPA shell + 隠しフォーム (no-JS / テスト fallback)。
  // POST /login への CSRF 検証は body の csrf を `csrfCookie` の値と
  // 二重提出パターンで照合する。
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
