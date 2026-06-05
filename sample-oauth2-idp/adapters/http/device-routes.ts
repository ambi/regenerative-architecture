/**
 * Layer 4 — Adapter Layer（HTTP: Device Authorization Grant, RFC 8628）
 *
 *   POST /device_authorization   device_code / user_code を発行 (§3.1)
 *   GET  /device                 user_code 入力フォーム (verification_uri)
 *   POST /device                 ユーザーによる承認 / 拒否 (§3.3)
 *
 * /token の device_code グラント分岐は token-routes.ts にある。
 *
 * 本サンプルではユーザー認証を簡略化 (X-User-Sub ヘッダー)。
 * 本番ではセッション Cookie・ログイン UI・CSRF トークンを実装する。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/domain/errors'
import { authenticateClient, type ClientAuthOptions } from './client-authentication'
import { oauthErrorResponse } from './error-response'
import { requestDeviceAuthorizationUseCase } from '../../src/usecases/request-device-authorization'
import { verifyUserCodeUseCase } from '../../src/usecases/verify-user-code'
import type { ClientRepository } from '../../src/ports/client-repository'
import type { UserRepository } from '../../src/ports/user-repository'
import type { DeviceCodeStore } from '../../src/ports/device-code-store'
import type { ClientAssertionReplayStore } from '../../src/ports/client-assertion-replay-store'
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

  // verification_uri — user_code 入力フォーム
  app.get('/device', (c) => {
    const prefill = c.req.query('user_code') ?? ''
    return c.html(verificationPage(prefill))
  })

  // ユーザーによる承認 / 拒否
  app.post('/device', async (c) => {
    try {
      const form = await c.req.parseBody()
      const user_code = String(form.user_code ?? '')
      const action = String(form.action ?? '') === 'deny' ? 'deny' : 'allow'

      const sub = c.req.header('X-User-Sub')
      if (!sub) return c.html(loginPage(user_code), 401)
      const user = await deps.userRepo.findBySub(sub)
      if (!user) return c.html(loginPage(user_code), 401)

      const { result } = await verifyUserCodeUseCase(
        { deviceCodeStore: deps.deviceCodeStore },
        { user_code, sub, auth_time: Math.floor(Date.now() / 1000), action },
        deps.emit,
      )
      return c.html(resultPage(result))
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}

function verificationPage(prefill: string): string {
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>デバイス認可</title></head>
<body>
<h1>デバイスを認可</h1>
<p>デバイスに表示された user_code を入力してください。</p>
<p>本サンプルでは X-User-Sub ヘッダーでログイン済みユーザーを識別します。</p>
<form method="POST" action="/device">
  <label>user_code: <input name="user_code" value="${escapeHtml(prefill)}"></label>
  <button type="submit" name="action" value="allow">許可する</button>
  <button type="submit" name="action" value="deny">拒否する</button>
</form>
</body></html>`
}

function loginPage(userCode: string): string {
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>ログイン</title></head>
<body>
<h1>ログインが必要です</h1>
<p>本サンプルでは X-User-Sub ヘッダーをユーザー識別に使用します。</p>
<pre>curl -X POST -H "X-User-Sub: user_alice" \\
  -d "user_code=${escapeHtml(userCode)}&amp;action=allow" .../device</pre>
</body></html>`
}

function resultPage(result: 'approved' | 'denied'): string {
  const msg =
    result === 'approved'
      ? 'デバイスを認可しました。デバイスに戻ってください。'
      : '認可を拒否しました。'
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>完了</title></head>
<body><h1>${msg}</h1></body></html>`
}

function escapeHtml(s: string): string {
  return s.replace(
    /[&<>"']/g,
    (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch] ?? ch,
  )
}
