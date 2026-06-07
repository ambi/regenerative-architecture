/**
 * Layer 4 — Adapter Layer（HTTP: /authorize と consent UI）
 *
 * 認可エンドポイント。
 * OAuth2/OIDC 側は AuthenticationContext を受け取り、認可判断・同意・
 * コード発行に集中する。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import {
  authorizeRequestUseCase,
  completeAuthenticationUseCase,
  grantConsentUseCase,
} from '../../src/oauth2/usecases/authorize-request'
import { issueAuthorizationCodeUseCase } from '../../src/oauth2/usecases/issue-authorization-code'
import { oauthErrorResponse } from './error-response'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { ConsentRepository } from '../../src/oauth2/ports/consent-repository'
import { renderShell } from './spa-shell'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from '../../src/oauth2/ports/authorization-store'
import {
  AuthenticationContextError,
  type AuthenticationContext,
  type AuthenticationContextResolver,
} from '../../src/authentication/domain/authentication-context'
import type { LoginContinuation } from '../../src/authentication/ports/login-continuation'
import {
  SESSION_COOKIE,
  type SessionManager,
} from '../../src/authentication/usecases/session-manager'
import type { AuthorizationRequest, Client, DomainEvent } from '../../src/spec-bindings/schemas'
import { loginRequiredResponse } from './authentication-routes'
import {
  assertCsrf,
  clearCookie,
  createCsrfToken,
  csrfCookie,
  WebSecurityError,
} from '../../src/shared/web-security'

export interface AuthorizeRoutesDeps {
  /** AS Issuer Identification (RFC 9207) で認可レスポンスに含める iss 値。 */
  issuer: string
  clientRepo: ClientRepository
  consentRepo: ConsentRepository
  requestStore: AuthorizationRequestStore
  codeStore: AuthorizationCodeStore
  parStore: PARStore
  authenticationContextResolver: AuthenticationContextResolver
  sessionManager: SessionManager
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

      // client_id / redirect_uri 不在は redirect 不能 → error response。
      // code_challenge の有無は policy (pkce_present) が require_pkce 込みで判定する。
      const required = ['client_id', 'redirect_uri', 'response_type']
      for (const k of required) {
        if (!params[k]) throw new OAuthError('invalid_request', `${k} が必要です`)
      }
      if (params.response_type !== 'code') {
        throw new OAuthError('unsupported_response_type', 'code のみサポート')
      }
      if (params.code_challenge && params.code_challenge_method !== 'S256') {
        throw new OAuthError('invalid_request', 'code_challenge_method は S256 のみ')
      }

      const { request, client } = await authorizeRequestUseCase(deps, {
        client_id: params.client_id,
        redirect_uri: params.redirect_uri,
        response_type: 'code',
        scope: params.scope ?? 'openid',
        state_param: params.state,
        nonce: params.nonce,
        code_challenge: params.code_challenge || undefined,
        code_challenge_method: params.code_challenge ? 'S256' : undefined,
        prompt: params.prompt,
        max_age: params.max_age ? Number(params.max_age) : undefined,
        id_token_hint: params.id_token_hint,
        par_used: parUsed,
      })

      const context = await deps.authenticationContextResolver.resolve(c.req.raw.headers)
      const acceptLanguage = c.req.header('accept-language')
      if (!context) {
        if (request.prompt === 'none') {
          throw new OAuthError('access_denied', 'prompt=none では対話的ログインを開始できません')
        }
        return loginRequiredResponse(request.id, acceptLanguage)
      }

      return await completeAuthorizedRequest(deps, request, client, context, {}, acceptLanguage)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      if (e instanceof AuthenticationContextError || e instanceof WebSecurityError) {
        return oauthErrorResponse(c, new OAuthError('invalid_request', e.message))
      }
      throw e
    }
  })

  app.get('/end_session', async (c) => {
    try {
      return await handleEndSession(
        deps,
        {
          ...Object.fromEntries(new URL(c.req.url).searchParams.entries()),
          acceptLanguage: c.req.header('accept-language'),
        },
        c.req.header('Cookie'),
      )
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      if (e instanceof WebSecurityError) {
        return oauthErrorResponse(c, new OAuthError('invalid_request', e.message))
      }
      throw e
    }
  })

  app.post('/end_session', async (c) => {
    try {
      const body = await c.req.parseBody()
      return await handleEndSession(
        deps,
        {
          client_id: stringBody(body.client_id),
          id_token_hint: stringBody(body.id_token_hint),
          post_logout_redirect_uri: stringBody(body.post_logout_redirect_uri),
          state: stringBody(body.state),
          acceptLanguage: c.req.header('accept-language'),
        },
        c.req.header('Cookie'),
      )
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
      assertCsrf(c.req.header('Cookie'), String(body.csrf ?? ''))
      const req = await deps.requestStore.find(request_id)
      if (!req) throw new OAuthError('invalid_request', '不明な認可リクエスト')

      if (action !== 'allow') {
        // 拒否ならエラーリダイレクト
        const url = new URL(req.redirect_uri)
        url.searchParams.set('error', 'access_denied')
        if (req.state_param) url.searchParams.set('state', req.state_param)
        url.searchParams.set('iss', deps.issuer) // RFC 9207
        return c.redirect(url.toString(), 302)
      }

      // 拒否でない: 一旦同意成立、認可コード発行
      // (acceptLanguage は本パスでは shell を返さないので不要)
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
      url.searchParams.set('iss', deps.issuer) // RFC 9207
      return c.redirect(url.toString(), 302)
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

export function createAuthorizationLoginContinuation(deps: AuthorizeRoutesDeps): LoginContinuation {
  return {
    async continueAfterLogin(
      requestId: string,
      context: AuthenticationContext,
      options: { promptLoginSatisfied?: boolean; acceptLanguage?: string } = {},
    ): Promise<Response> {
      const req = await deps.requestStore.find(requestId)
      if (!req) throw new OAuthError('invalid_request', '不明な認可リクエスト')
      const client = await deps.clientRepo.findById(req.client_id)
      if (!client) throw new OAuthError('invalid_request', '不明なクライアント')
      return await completeAuthorizedRequest(
        deps,
        req,
        client,
        context,
        { promptLoginSatisfied: options.promptLoginSatisfied },
        options.acceptLanguage,
      )
    },
  }
}

function stringBody(value: unknown): string | undefined {
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

async function handleEndSession(
  deps: AuthorizeRoutesDeps,
  params: {
    client_id?: string
    id_token_hint?: string
    post_logout_redirect_uri?: string
    state?: string
    acceptLanguage?: string
  },
  cookieHeader: string | undefined,
): Promise<Response> {
  await deps.sessionManager.revoke(cookieHeader)

  if (!params.post_logout_redirect_uri) {
    return new Response(loggedOutShell(params.id_token_hint, params.acceptLanguage), {
      status: 200,
      headers: {
        'content-type': 'text/html; charset=UTF-8',
        'set-cookie': clearCookie(SESSION_COOKIE),
      },
    })
  }

  if (!params.client_id) {
    throw new OAuthError(
      'invalid_request',
      'post_logout_redirect_uri の検証には client_id が必要です',
    )
  }

  const client = await deps.clientRepo.findById(params.client_id)
  if (!client) {
    throw new OAuthError('invalid_request', '未知の client_id です')
  }
  if (!client.redirect_uris.includes(params.post_logout_redirect_uri)) {
    throw new OAuthError(
      'invalid_request',
      'post_logout_redirect_uri が登録済み URI ではありません',
    )
  }

  const url = new URL(params.post_logout_redirect_uri)
  if (params.state) url.searchParams.set('state', params.state)
  const response = Response.redirect(url.toString(), 302)
  response.headers.append('set-cookie', clearCookie(SESSION_COOKIE))
  return response
}

async function completeAuthorizedRequest(
  deps: AuthorizeRoutesDeps,
  request: AuthorizationRequest,
  client: Client,
  context: AuthenticationContext,
  options: { promptLoginSatisfied?: boolean } = {},
  acceptLanguage?: string,
): Promise<Response> {
  const {
    request: postAuth,
    needsConsent,
    needsAuthentication,
  } = await completeAuthenticationUseCase(
    deps,
    request,
    context.sub,
    new Date(context.auth_time * 1000),
    new Date(),
    options,
  )

  if (needsAuthentication) {
    if (request.prompt === 'none') {
      throw new OAuthError('access_denied', 'prompt=none では再認証を開始できません')
    }
    return loginRequiredResponse(request.id, acceptLanguage)
  }

  if (needsConsent) {
    return consentResponse(postAuth, client, acceptLanguage)
  }

  const { code } = await issueAuthorizationCodeUseCase(deps, postAuth)
  deps.emit({
    type: 'AuthorizationCodeIssued',
    occurredAt: new Date().toISOString(),
    clientId: client.client_id,
    sub: context.sub,
    scopes: code.scopes,
    codeChallengeMethod: code.code_challenge_method,
  })

  const url = new URL(postAuth.redirect_uri)
  url.searchParams.set('code', code.code)
  if (postAuth.state_param) url.searchParams.set('state', postAuth.state_param)
  url.searchParams.set('iss', deps.issuer) // RFC 9207
  return Response.redirect(url.toString(), 302)
}

function consentResponse(
  req: AuthorizationRequest,
  client: Client,
  acceptLanguage?: string,
): Response {
  const csrf = createCsrfToken()
  const html = renderShell({
    page: 'consent',
    title: '同意',
    meta: {
      'request-id': req.id,
      csrf,
      'client-id': client.client_id,
      'client-name': client.client_name ?? client.client_id,
      scope: req.scope,
    },
    fallbackForm: {
      action: '/consent',
      fields: { request_id: req.id, csrf },
      buttons: [
        { name: 'action', value: 'allow', label: '許可する' },
        { name: 'action', value: 'deny', label: '拒否する' },
      ],
    },
    acceptLanguage,
  })
  return new Response(html, {
    status: 200,
    headers: {
      'content-type': 'text/html; charset=UTF-8',
      'set-cookie': csrfCookie(csrf),
    },
  })
}

function loggedOutShell(idTokenHint?: string, acceptLanguage?: string): string {
  return renderShell({
    page: 'error',
    title: 'ログアウトしました',
    meta: {
      'error-kind': 'logged_out',
      'error-title': 'ログアウトしました',
      'error-description': 'セッションを終了しました。安全のためブラウザを閉じてください。',
      ...(idTokenHint ? { 'error-detail': 'id_token_hint を受け取りました' } : {}),
    },
    acceptLanguage,
  })
}
