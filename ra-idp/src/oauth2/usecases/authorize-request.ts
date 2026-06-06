/**
 * Layer 3 — Application Logic
 *
 * /authorize エンドポイントの中核ロジック。
 * - クライアントと redirect_uri を検証
 * - スコープがクライアント宣言の部分集合か検証
 * - PKCE が存在するか検証
 * - FAPI クライアントなら PAR 経由か検証
 *
 * 認可は AuthZEN ポリシー（spec/scl.yaml permissions、評価器は src/spec-bindings/policy/）に委譲する。
 */

import { evaluate } from '../../spec-bindings/policy/client-authorization'
import type { Client, Consent } from '../../spec-bindings/schemas'
import {
  createAuthorizationRequest,
  advance,
  type AuthorizationRequest,
  type CreateAuthorizationRequestInput,
} from '../domain/authorization-request'
import type { ClientRepository } from '../ports/client-repository'
import type { ConsentRepository } from '../ports/consent-repository'
import type { AuthorizationRequestStore } from '../ports/authorization-store'
import { OAuthError } from '../protocol/oauth-error'

/**
 * client metadata `require_pkce` の解決規則 (ADR-002 改訂 / RFC 9700 / OAuth 2.1)。
 * - 明示 true/false が登録されていればそれを採用
 * - 未指定なら client_type と fapi_profile から決める:
 *   - public client → true (PKCE が唯一の防御)
 *   - FAPI client → true (FAPI 2.0 §5.1 で MUST)
 *   - confidential client → false (RFC 9700 推奨だが原仕様では任意。明示 opt-in 設計)
 */
export function resolveRequirePkce(client: Client): boolean {
  if (typeof client.require_pkce === 'boolean') return client.require_pkce
  if (client.client_type === 'public') return true
  if (client.fapi_profile && client.fapi_profile !== 'none') return true
  return false
}

export interface AuthorizeInput extends CreateAuthorizationRequestInput {
  /** PAR を経由したか。アダプター層が判定する */
  par_used: boolean
}

export interface AuthorizeResult {
  request: AuthorizationRequest
  client: Client
  /** 既存コンセントが要求スコープを覆っていれば true → コンセント UI をスキップ可 */
  existing_consent_covers_scopes: boolean
}

export async function authorizeRequestUseCase(
  deps: {
    clientRepo: ClientRepository
    consentRepo: ConsentRepository
    requestStore: AuthorizationRequestStore
  },
  input: AuthorizeInput,
): Promise<AuthorizeResult> {
  const client = await deps.clientRepo.findById(input.client_id)
  if (!client) {
    throw new OAuthError('unauthorized_client', '未登録のクライアントです')
  }

  const requestedScopes = input.scope.split(/\s+/).filter(Boolean)
  const clientScopes = client.scope.split(/\s+/).filter(Boolean)

  const decision = evaluate({
    subject: {
      type: 'Client',
      id: client.client_id,
      properties: {
        clientType: client.client_type,
        scopes: clientScopes,
        redirectUris: client.redirect_uris,
        requirePAR: client.require_pushed_authorization_requests,
        requirePkce: resolveRequirePkce(client),
      },
    },
    action: { name: 'authorize:initiate' },
    resource: {
      type: 'AuthorizationRequest',
      properties: {
        codeChallenge: input.code_challenge,
        scopes: requestedScopes,
      },
    },
    context: {
      redirectUri: input.redirect_uri,
      parUsed: input.par_used,
    },
  })

  if (decision.decision === 'Deny') {
    throw new OAuthError(
      'invalid_request',
      `認可開始が拒否されました: ${decision.reasons?.join(', ')}`,
    )
  }

  // 認可リクエストを「received」状態で作成し、validate イベントで authentication_pending へ進める
  let req = createAuthorizationRequest(input)
  req = advance(req, 'validate')
  await deps.requestStore.save(req)

  // 既存コンセントが要求スコープを覆っているかチェック
  // (この時点ではユーザーがまだ未認証なので、認証後に再評価する)
  const _consent: Consent | null = null
  return {
    request: req,
    client,
    existing_consent_covers_scopes: false, // 認証前なので未判定
  }
}

/**
 * ユーザー認証成功後に呼ばれる。
 * 既存コンセントの有無に応じて、consented まで一気に進めるか consent_pending に留めるかを決める。
 */
export async function completeAuthenticationUseCase(
  deps: { consentRepo: ConsentRepository; requestStore: AuthorizationRequestStore },
  req: AuthorizationRequest,
  authenticated_sub: string,
  authTime: Date = new Date(),
  now: Date = new Date(),
  options: { promptLoginSatisfied?: boolean } = {},
): Promise<{ request: AuthorizationRequest; needsConsent: boolean; needsAuthentication: boolean }> {
  const authTimeSeconds = Math.floor(authTime.getTime() / 1000)
  const nowSeconds = Math.floor(now.getTime() / 1000)

  if (req.prompt === 'login' && !options.promptLoginSatisfied) {
    await deps.requestStore.save(req)
    return { request: req, needsConsent: false, needsAuthentication: true }
  }

  if (req.max_age !== undefined && nowSeconds - authTimeSeconds >= req.max_age) {
    await deps.requestStore.save(req)
    return { request: req, needsConsent: false, needsAuthentication: true }
  }

  let next = advance(req, 'authenticate_user', {
    sub: authenticated_sub,
    auth_time: authTimeSeconds,
  })

  const requestedScopes = req.scope.split(/\s+/).filter(Boolean)
  const consent = await deps.consentRepo.find(authenticated_sub, req.client_id)
  const covered =
    !!consent &&
    !consent.revoked_at &&
    Date.parse(consent.expires_at) > Date.now() &&
    requestedScopes.every((s) => consent.scopes.includes(s))

  if (covered && req.prompt !== 'consent') {
    // 既存コンセントで OK → consent_pending を経由せずに consented へ
    // 状態機械では authenticated → issue_code が許可されているので、そのまま issue_code で
    // code_issued まで進めるのは usecase 側 (issueAuthorizationCode) に任せる
    next = advance(next, 'request_consent') // → consent_pending
    next = advance(next, 'grant_consent') // → consented
    await deps.requestStore.save(next)
    return { request: next, needsConsent: false, needsAuthentication: false }
  }

  next = advance(next, 'request_consent') // → consent_pending
  await deps.requestStore.save(next)
  return { request: next, needsConsent: true, needsAuthentication: false }
}

/**
 * コンセント UI でユーザーが grant_consent を押した後に呼ばれる。
 */
export async function grantConsentUseCase(
  deps: { consentRepo: ConsentRepository; requestStore: AuthorizationRequestStore },
  req: AuthorizationRequest,
  scopes: string[],
  now: Date = new Date(),
): Promise<AuthorizationRequest> {
  const next = advance(req, 'grant_consent')
  // コンセントを永続化
  await deps.consentRepo.save({
    sub: req.sub!,
    client_id: req.client_id,
    scopes,
    granted_at: now.toISOString(),
    expires_at: new Date(now.getTime() + 365 * 24 * 3600 * 1000).toISOString(),
  })
  await deps.requestStore.save(next)
  return next
}
