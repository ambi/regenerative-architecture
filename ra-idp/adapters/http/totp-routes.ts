/**
 * Layer 4 — Adapter Layer (HTTP: /totp)
 *
 * パスワード成功後 authentication_pending=true となった LoginSession に対し、
 * Authenticator アプリの 6 桁コードで第二要素検証を行う form-based エンドポイント。
 *
 * - GET /totp: SPA shell + hidden form
 * - POST /totp: csrf 検証 → セッション解決 → verifyTotpFactorUseCase →
 *               session.completeFactor で amr に 'otp' を足して acr を mfa に昇格 →
 *               OAuth2/OIDC continuation
 *
 * 他 factor (WebAuthn 等) は HTTP 形が異なるため、本ファイルとは別の adapter として
 * 独立に実装する。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { LoginContinuation } from '../../src/authentication/ports/login-continuation'
import type { MfaFactorRepository } from '../../src/authentication/ports/mfa-factor-repository'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import {
  SESSION_COOKIE,
  SESSION_TTL_SECONDS,
  type SessionManager,
} from '../../src/authentication/usecases/session-manager'
import { verifyTotpFactorUseCase } from '../../src/authentication/usecases/verify-totp-factor'
import {
  assertCsrf,
  createCsrfToken,
  csrfCookie,
  sessionCookie,
  WebSecurityError,
} from '../../src/shared/web-security'
import { oauthErrorResponse } from './error-response'
import { renderShell } from './spa-shell'

export interface TotpRoutesDeps {
  sessionManager: SessionManager
  mfaFactorRepo: MfaFactorRepository
  continuation: LoginContinuation
  emit: (e: DomainEvent) => void
}

export function createTotpRoutes(deps: TotpRoutesDeps) {
  const app = new Hono()

  app.get('/totp', async (c) => {
    const requestId = c.req.query('request_id') ?? ''
    // POST /totp 成功直後に SPA が `window.location.reload()` でこのページに戻ってくる
    // ケースがある。アドレスバーは `/totp?request_id=...` のまま (LoginPage は
    // `/authorize` のままだが TotpPage は違う) なので、ここで session を見て、
    // 既に factor 検証が済んでいるなら OAuth2 continuation (consent shell or
    // callback redirect) に進める。そうしないとリロードで TOTP 画面が再表示され、
    // ユーザが古い code を再投入して期限切れエラーになる。
    if (requestId) {
      try {
        const context = await deps.sessionManager.resolve(c.req.raw.headers)
        if (
          context &&
          !context.authentication_pending &&
          context.amr.includes('otp')
        ) {
          return await deps.continuation.continueAfterLogin(requestId, context, {
            promptLoginSatisfied: true,
            acceptLanguage: c.req.header('accept-language'),
          })
        }
      } catch (e) {
        if (e instanceof OAuthError) return oauthErrorResponse(c, e)
        throw e
      }
    }
    return totpChallengeResponse(requestId, c.req.header('accept-language'))
  })

  app.post('/totp', async (c) => {
    try {
      const body = await c.req.parseBody()
      const requestId = String(body.request_id ?? '')
      const code = String(body.code ?? '')
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))

      const context = await deps.sessionManager.resolve(c.req.raw.headers)
      if (!context || !context.session_id) {
        throw new OAuthError('access_denied', 'TOTP 検証セッションが見つかりません')
      }
      // password 直後 (authentication_pending=true) もしくは acr_values による step-up
      // (authentication_pending=false だが amr に otp を含まない) のいずれかなら受理する。
      // 既に otp を含む場合は二重検証になるため拒否。
      const alreadyHasOtp = context.amr.includes('otp')
      if (alreadyHasOtp && !context.authentication_pending) {
        throw new OAuthError('access_denied', 'TOTP は既に検証済みです')
      }

      const result = await verifyTotpFactorUseCase(
        { mfaFactorRepo: deps.mfaFactorRepo },
        { sub: context.sub, code },
      )
      if (!result.ok) {
        deps.emit({
          type: 'AuthenticationFailed',
          occurredAt: new Date().toISOString(),
          username: context.sub,
          reason: result.reason === 'no_factor' ? 'no_factor' : 'invalid_totp',
        })
        return totpChallengeResponse(requestId, c.req.header('accept-language'), true)
      }

      const completed = await deps.sessionManager.completeFactor(context.session_id, ['otp'])
      if (!completed) {
        throw new OAuthError('access_denied', 'セッションが失効しました')
      }
      deps.emit({
        type: 'UserAuthenticated',
        occurredAt: new Date().toISOString(),
        sub: completed.sub,
        amr: completed.amr,
      })

      const response = await deps.continuation.continueAfterLogin(requestId, completed, {
        promptLoginSatisfied: true,
        acceptLanguage: c.req.header('accept-language'),
      })
      // session id は同じだが ストア側で amr/acr/authentication_pending を更新済み。
      // Cookie 寿命を延ばすため Set-Cookie を再発行する。
      response.headers.append(
        'set-cookie',
        sessionCookie(SESSION_COOKIE, completed.session_id ?? '', SESSION_TTL_SECONDS),
      )
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

/**
 * TOTP challenge ページの shell を組み立てる pure 関数。
 * authentication-routes (POST /login 直後) と authorize-routes (authentication_pending な
 * セッションで /authorize に来たとき) からも呼ばれる。redirect ではなく shell HTML を
 * 直接返すことで、ブラウザのアドレスバーを元の `/authorize` のまま維持し、SPA reload で
 * `/authorize` が再評価される (= LoginPage と同じ仕組み)。
 */
export function totpChallengeResponse(
  requestId: string,
  acceptLanguage?: string,
  invalid = false,
): Response {
  const csrf = createCsrfToken()
  const html = renderShell({
    page: 'totp',
    title: '第二要素の確認',
    meta: {
      'request-id': requestId,
      csrf,
      ...(invalid ? { 'totp-invalid': '1' } : {}),
    },
    fallbackForm: {
      action: '/totp',
      fields: { request_id: requestId, csrf },
    },
    acceptLanguage,
  })
  return new Response(html, {
    status: invalid ? 401 : 200,
    headers: {
      'content-type': 'text/html; charset=UTF-8',
      'set-cookie': csrfCookie(csrf),
    },
  })
}
