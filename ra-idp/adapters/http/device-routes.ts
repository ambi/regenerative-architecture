/**
 * Layer 4 — Adapter Layer（HTTP: Device Authorization Grant, RFC 8628）
 *
 *   POST /device_authorization   device_code / user_code を発行 (§3.1)
 *   GET  /device                 user_code 入力フォーム (verification_uri)
 *   POST /device                 ユーザーによる承認 / 拒否 (§3.3)
 *
 * /token の device_code グラント分岐は token-routes.ts にある。
 *
 * 本アプリではユーザー認証を簡略化 (X-User-Sub ヘッダー)。
 * 本番ではセッション Cookie・ログイン UI・CSRF トークンを実装する。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { authenticateClient, type ClientAuthOptions } from './client-authentication'
import { oauthErrorResponse } from './error-response'
import { renderShell } from './spa-shell'
import { createCsrfToken, csrfCookie } from '../../src/shared/web-security'
import { requestDeviceAuthorizationUseCase } from '../../src/oauth2/usecases/request-device-authorization'
import { verifyUserCodeUseCase } from '../../src/oauth2/usecases/verify-user-code'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { DeviceCodeStore } from '../../src/oauth2/ports/device-code-store'
import type { ClientAssertionReplayStore } from '../../src/oauth2/ports/client-assertion-replay-store'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export interface DeviceRoutesDeps {
  issuer: string
  clientRepo: ClientRepository
  userRepo: UserRepository
  deviceCodeStore: DeviceCodeStore
  clientAssertionReplayStore: ClientAssertionReplayStore
  emit: (e: DomainEvent) => void
}

export function createDeviceRoutes(deps: DeviceRoutesDeps) {
  const app = new Hono()
  const clientAuth: ClientAuthOptions = {
    issuer: deps.issuer,
    clientAssertionReplayStore: deps.clientAssertionReplayStore,
  }

  // §3.1 — device_code / user_code 発行
  app.post('/device_authorization', async (c) => {
    try {
      const body = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
      const auth = await authenticateClient(c, body, deps.clientRepo, clientAuth)
      const { response, record } = await requestDeviceAuthorizationUseCase(
        { deviceCodeStore: deps.deviceCodeStore, issuer: deps.issuer },
        { client: auth.client, scope: body.scope },
      )
      deps.emit({
        type: 'DeviceAuthorizationRequested',
        occurredAt: new Date().toISOString(),
        clientId: auth.client.client_id,
        scopes: record.scopes,
      })
      // RFC 8628 §3.2 のレスポンス
      return c.json({
        device_code: response.device_code,
        user_code: response.user_code,
        verification_uri: response.verification_uri,
        verification_uri_complete: response.verification_uri_complete,
        expires_in: response.expires_in,
        interval: response.interval,
      })
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  // verification_uri — user_code 入力フォーム (SPA shell)
  app.get('/device', (c) => {
    const prefill = c.req.query('user_code') ?? ''
    const csrf = createCsrfToken()
    const html = renderShell({
      page: 'device',
      title: 'デバイスを認可',
      meta: { 'user-code': prefill, csrf },
      fallbackForm: {
        action: '/device',
        fields: { user_code: prefill, csrf },
        buttons: [
          { name: 'action', value: 'allow', label: '認可する' },
          { name: 'action', value: 'deny', label: '拒否する' },
        ],
      },
      acceptLanguage: c.req.header('accept-language'),
    })
    return new Response(html, {
      status: 200,
      headers: { 'content-type': 'text/html; charset=UTF-8', 'set-cookie': csrfCookie(csrf) },
    })
  })

  // ユーザーによる承認 / 拒否
  app.post('/device', async (c) => {
    try {
      const form = await c.req.parseBody()
      const user_code = String(form.user_code ?? '')
      const action = String(form.action ?? '') === 'deny' ? 'deny' : 'allow'

      const acceptLanguage = c.req.header('accept-language')
      const sub = c.req.header('X-User-Sub')
      if (!sub) return deviceLoginRequired(user_code, acceptLanguage)
      const user = await deps.userRepo.findBySub(sub)
      if (!user) return deviceLoginRequired(user_code, acceptLanguage)

      const { result } = await verifyUserCodeUseCase(
        { deviceCodeStore: deps.deviceCodeStore },
        { user_code, sub, auth_time: Math.floor(Date.now() / 1000), action },
        deps.emit,
      )
      return new Response(
        renderShell({
          page: 'error',
          title: result === 'approved' ? 'デバイスを認可しました' : 'デバイス認可を拒否しました',
          meta: {
            'error-kind': result === 'approved' ? 'device_approved' : 'device_denied',
            'error-title':
              result === 'approved' ? 'デバイスを認可しました' : 'デバイス認可を拒否しました',
            'error-description':
              result === 'approved'
                ? 'デバイス側で続行できます。このタブは閉じてかまいません。'
                : 'デバイスからのアクセス要求を拒否しました。',
          },
          acceptLanguage,
        }),
        { status: 200, headers: { 'content-type': 'text/html; charset=UTF-8' } },
      )
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}

/**
 * `/device` POST に X-User-Sub が無いケース。デモ環境ではログイン UI を経由せず
 * ヘッダで sub を渡す前提のため、その案内を error shell として返す。
 */
function deviceLoginRequired(userCode: string, acceptLanguage?: string): Response {
  return new Response(
    renderShell({
      page: 'error',
      title: 'ログインが必要です',
      meta: {
        'error-kind': 'login_required',
        'error-title': 'ログインが必要です',
        'error-description':
          'デモ環境では X-User-Sub ヘッダでユーザー識別を行います。詳細は README を参照してください。',
        'error-detail': `user_code=${userCode}`,
      },
      acceptLanguage,
    }),
    { status: 401, headers: { 'content-type': 'text/html; charset=UTF-8' } },
  )
}
