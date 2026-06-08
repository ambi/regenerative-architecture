/**
 * Layer 4 — Adapter Layer (HTTP: /totp)
 *
 * パスワード成功後 authentication_pending=true となった LoginSession に対し、
 * Authenticator アプリの 6 桁コードで第二要素検証を行う form-based エンドポイント。
 *
 * - GET /totp: SPA shell + hidden form fallback
 * - POST /api/auth/totp: JSON API。csrf 検証 → セッション解決 → verifyTotpFactorUseCase →
 *                        session.completeFactor で amr に 'otp' を足して acr を mfa に昇格 →
 *                        OAuth2/OIDC continuation
 * - POST /totp: no-JS/form fallback。
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
import { clearTransactionCookie, transactionIdFromCookie, noStoreJSON } from './browser-transaction'

export interface TotpRoutesDeps {
  sessionManager: SessionManager
  mfaFactorRepo: MfaFactorRepository
  continuation: LoginContinuation
  emit: (e: DomainEvent) => void
}

export function createTotpRoutes(deps: TotpRoutesDeps) {
  const app = new Hono()

  app.get('/totp', async (c) => {
    const requestId =
      transactionIdFromCookie(c.req.header('Cookie')) || c.req.query('request_id') || ''
    // no-JS/form fallback で factor 検証済みの状態から GET /totp に戻った場合は、
    // 既に factor 検証が済んでいるなら OAuth2 continuation に進める。
    if (requestId) {
      try {
        const context = await deps.sessionManager.resolve(c.req.raw.headers)
        if (context && !context.authentication_pending && context.amr.includes('otp')) {
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

  app.post('/api/auth/totp', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const requestId = transactionIdFromCookie(c.req.header('Cookie'))
      if (!requestId) {
        return noStoreJSON(c, 401, {
          error: 'transaction_unavailable',
          message: '認可トランザクションがありません',
        })
      }
      const body = await c.req.json().catch(() => null)
      const code = typeof body?.code === 'string' ? body.code : ''
      const response = await completeTotp(
        deps,
        requestId,
        code,
        c.req.raw.headers,
        c.req.header('accept-language'),
      )
      return responseToBrowserFlow(response)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      if (e instanceof WebSecurityError) {
        return noStoreJSON(c, 403, { error: 'csrf_failed', message: e.message })
      }
      throw e
    }
  })

  app.post('/totp', async (c) => {
    try {
      const body = await c.req.parseBody()
      const requestId = String(body.request_id ?? '')
      const code = String(body.code ?? '')
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))

      return await completeTotp(
        deps,
        requestId,
        code,
        c.req.raw.headers,
        c.req.header('accept-language'),
      )
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

async function completeTotp(
  deps: TotpRoutesDeps,
  requestId: string,
  code: string,
  headers: Headers,
  acceptLanguage?: string,
): Promise<Response> {
  const context = await deps.sessionManager.resolve(headers)
  if (!context?.session_id) {
    throw new OAuthError('access_denied', 'TOTP 検証セッションが見つかりません')
  }
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
    return totpChallengeResponse(requestId, undefined, true)
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
    acceptLanguage,
  })
  response.headers.append(
    'set-cookie',
    sessionCookie(SESSION_COOKIE, completed.session_id ?? '', SESSION_TTL_SECONDS),
  )
  return response
}

function responseToBrowserFlow(response: Response): Response {
  const location = response.headers.get('location')
  if (location) {
    return copySetCookie(response, browserFlowJSON({ redirect_to: location }, true))
  }
  if (response.status === 401) {
    return copySetCookie(
      response,
      new Response(
        JSON.stringify({ error: 'invalid_totp', message: 'TOTPコードを確認してください。' }),
        {
          status: 401,
          headers: {
            'content-type': 'application/json; charset=UTF-8',
            'cache-control': 'no-store',
          },
        },
      ),
    )
  }
  if (response.headers.get('content-type')?.includes('text/html')) {
    return copySetCookie(response, browserFlowJSON({ next: '/consent' }))
  }
  return response
}

function browserFlowJSON(
  body: { next?: string; redirect_to?: string },
  clearTransaction = false,
): Response {
  const response = new Response(JSON.stringify(body), {
    status: 200,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
  if (clearTransaction) {
    response.headers.append('set-cookie', clearTransactionCookie())
  }
  return response
}

function copySetCookie(from: Response, to: Response): Response {
  for (const setCookie of from.headers.getSetCookie()) {
    to.headers.append('set-cookie', setCookie)
  }
  return to
}

/**
 * TOTP challenge ページの shell を組み立てる pure 関数。
 * `/authorize` はこの shell を直接返さず `/totp` に redirect する。
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
