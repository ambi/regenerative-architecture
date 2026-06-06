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
import type { PasswordVerifier } from '../../src/authentication/usecases/password-verifier'
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

export interface AuthenticationRoutesDeps {
  userRepo: UserRepository
  passwordVerifier: PasswordVerifier
  sessionManager: SessionManager
  continuation: LoginContinuation
  emit: (e: DomainEvent) => void
}

export function createAuthenticationRoutes(deps: AuthenticationRoutesDeps) {
  const app = new Hono()

  app.post('/login', async (c) => {
    try {
      const body = await c.req.parseBody()
      const requestId = String(body.request_id ?? '')
      const username = String(body.username ?? '')
      const password = String(body.password ?? '')
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))

      const user = await deps.userRepo.findByUsername(username)
      if (!user || !deps.passwordVerifier.verify(password, user.password_hash)) {
        deps.emit({
          type: 'AuthenticationFailed',
          occurredAt: new Date().toISOString(),
          username,
          reason: 'invalid_credentials',
        })
        return loginRequiredResponse(requestId)
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

export function loginRequiredResponse(requestId: string): Response {
  const csrf = createCsrfToken()
  return new Response(loginPage(requestId, csrf), {
    status: 401,
    headers: {
      'content-type': 'text/html; charset=UTF-8',
      'set-cookie': csrfCookie(csrf),
    },
  })
}

function loginPage(requestId: string, csrf: string): string {
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>ログイン</title></head>
<body>
<h1>ログインが必要です</h1>
<form method="POST" action="/login">
  <input type="hidden" name="request_id" value="${escapeHtml(requestId)}">
  <input type="hidden" name="csrf" value="${escapeHtml(csrf)}">
  <label>ユーザー名 <input name="username" autocomplete="username" required></label>
  <label>パスワード <input name="password" type="password" autocomplete="current-password" required></label>
  <button type="submit">ログイン</button>
</form>
</body></html>`
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}
