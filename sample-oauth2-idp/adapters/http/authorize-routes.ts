/**
 * Layer 4 — Adapter Layer（HTTP: /authorize と consent UI）
 *
 * 認可エンドポイント。
 * 本サンプルではユーザー認証を簡略化（X-User ヘッダーでログイン済みとみなす）し、
 * コンセント UI も最小限の HTML を返す。
 *
 * 本番では:
 *   - セッション Cookie でユーザー識別
 *   - ログイン UI / MFA 等を経由
 *   - CSRF トークン
 * を実装する必要がある。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/domain/errors'
import {
  authorizeRequestUseCase,
  completeAuthenticationUseCase,
  grantConsentUseCase,
} from '../../src/usecases/authorize-request'
import { issueAuthorizationCodeUseCase } from '../../src/usecases/issue-authorization-code'
import { oauthErrorResponse } from './error-response'
import type { ClientRepository } from '../../src/ports/client-repository'
import type { UserRepository } from '../../src/ports/user-repository'
import type { ConsentRepository } from '../../src/ports/consent-repository'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from '../../src/ports/authorization-store'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export interface AuthorizeRoutesDeps {
  clientRepo: ClientRepository
  userRepo: UserRepository
  consentRepo: ConsentRepository
  requestStore: AuthorizationRequestStore
  codeStore: AuthorizationCodeStore
  parStore: PARStore
  emit: (e: DomainEvent) => void
}

export function createAuthorizeRoutes(deps: AuthorizeRoutesDeps) {
  const app = new Hono()

  app.get('/authorize', async (c) => {
    try {
      let params: Record<string, string> = Object.fromEntries(
        new URL(c.req.url).searchParams.entries(),
      )

      // PAR を経由している場合、保存されたパラメータで上書きする (RFC 9126 §4)
      let parUsed = false
      if (params.request_uri) {
        const consumed = await deps.parStore.consume(params.request_uri)
        if (!consumed) {
          throw new OAuthError('invalid_request_uri', 'request_uri が無効または使用済み')
        }
        // RFC 9126 §4: クエリの client_id は任意、与えられた場合は PAR レコードと一致する必要がある
        if (params.client_id && consumed.client_id !== params.client_id) {
          throw new OAuthError('invalid_request', 'client_id が PAR レコードと一致しません')
        }
        // PAR レコードの正規 client_id を権威とする（保存時に分離されているため復元する）
        params = { ...consumed.parameters, client_id: consumed.client_id }
        parUsed = true
      }

      const required = ['client_id', 'redirect_uri', 'response_type', 'code_challenge']
      for (const k of required) {
        if (!params[k]) throw new OAuthError('invalid_request', `${k} が必要です`)
      }
      if (params.response_type !== 'code') {
        throw new OAuthError('unsupported_response_type', 'code のみサポート')
      }
      if (params.code_challenge_method !== 'S256') {
        throw new OAuthError('invalid_request', 'code_challenge_method は S256 のみ')
      }

      const { request, client } = await authorizeRequestUseCase(deps, {
        client_id: params.client_id,
        redirect_uri: params.redirect_uri,
        response_type: 'code',
        scope: params.scope ?? 'openid',
        state_param: params.state,
        nonce: params.nonce,
        code_challenge: params.code_challenge,
        code_challenge_method: 'S256',
        prompt: params.prompt,
        max_age: params.max_age ? Number(params.max_age) : undefined,
        par_used: parUsed,
      })

      // ユーザー認証（本サンプル: ヘッダー由来）
      const sub = c.req.header('X-User-Sub')
      if (!sub) {
        return c.html(loginPage(request.id), 401)
      }
      const user = await deps.userRepo.findBySub(sub)
      if (!user) {
        return c.html(loginPage(request.id), 401)
      }

      const { request: postAuth, needsConsent } = await completeAuthenticationUseCase(
        deps,
        request,
        sub,
      )

      if (needsConsent) {
        return c.html(consentPage(postAuth, client))
      }

      // consented or skipped → 認可コード発行
      const { code } = await issueAuthorizationCodeUseCase(deps, postAuth)
      deps.emit({
        type: 'AuthorizationCodeIssued',
        occurredAt: new Date().toISOString(),
        clientId: client.client_id,
        sub,
        scopes: code.scopes,
        codeChallengeMethod: code.code_challenge_method,
      })

      const url = new URL(postAuth.redirect_uri)
      url.searchParams.set('code', code.code)
      if (postAuth.state_param) url.searchParams.set('state', postAuth.state_param)
      return c.redirect(url.toString(), 302)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  // コンセント画面の POST を受ける
  app.post('/consent', async (c) => {
    try {
      const body = await c.req.parseBody()
      const request_id = String(body.request_id ?? '')
      const action = String(body.action ?? '')
      const req = await deps.requestStore.find(request_id)
      if (!req) throw new OAuthError('invalid_request', '不明な認可リクエスト')

      if (action !== 'allow') {
        // 拒否ならエラーリダイレクト
        const url = new URL(req.redirect_uri)
        url.searchParams.set('error', 'access_denied')
        if (req.state_param) url.searchParams.set('state', req.state_param)
        return c.redirect(url.toString(), 302)
      }

      const scopes = req.scope.split(/\s+/).filter(Boolean)
      const consented = await grantConsentUseCase(deps, req, scopes)
      const { code } = await issueAuthorizationCodeUseCase(deps, consented)
      deps.emit({
        type: 'AuthorizationCodeIssued',
        occurredAt: new Date().toISOString(),
        clientId: req.client_id,
        sub: req.sub!,
        scopes,
        codeChallengeMethod: req.code_challenge_method,
      })

      const url = new URL(req.redirect_uri)
      url.searchParams.set('code', code.code)
      if (req.state_param) url.searchParams.set('state', req.state_param)
      return c.redirect(url.toString(), 302)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}

function loginPage(requestId: string): string {
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>ログイン</title></head>
<body>
<h1>ログインが必要です</h1>
<p>本サンプルでは X-User-Sub ヘッダーをユーザー識別に使用します。</p>
<pre>curl -H "X-User-Sub: user_alice" ".../authorize?...&amp;state=..."</pre>
<p>request_id: ${requestId}</p>
</body></html>`
}

function consentPage(
  req: { id: string; client_id: string; scope: string },
  client: { client_name?: string; client_id: string },
): string {
  return `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>同意</title></head>
<body>
<h1>${client.client_name ?? client.client_id} があなたの情報へのアクセスを要求しています</h1>
<p>要求スコープ: <code>${req.scope}</code></p>
<form method="POST" action="/consent">
  <input type="hidden" name="request_id" value="${req.id}">
  <button type="submit" name="action" value="allow">許可する</button>
  <button type="submit" name="action" value="deny">拒否する</button>
</form>
</body></html>`
}
