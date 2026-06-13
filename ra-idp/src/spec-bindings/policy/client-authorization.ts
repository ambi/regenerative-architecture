/**
 * Layer 3 — Specification Binding (TypeScript)（認可ポリシー）
 *
 * 仕様本体は spec/scl.yaml の `permissions` セクション（PascalCase 宣言）。
 * このファイルは AuthZEN スタイルの evaluate() を提供する TypeScript バインディング。
 *
 * SCL 上の `allow_when` 式を、ここで「名前付きルールの集合」に分解する。
 * 名前付きルールは下流アダプター（Cedar / OPA / Cerbos）へのマッピング点となる。
 * 別言語に移植する場合は SCL を直接消費し、本ディレクトリを該当言語版で置き換える。
 *
 * ADR-010 参照。
 */

import { scl } from '../scl'

// ---------------------------------------------------------------
// AuthZEN リクエスト・レスポンス型（OpenID Foundation AuthZEN 準拠）
// ---------------------------------------------------------------

export interface AuthZENSubject {
  type: 'Client' | 'User'
  id: string
  properties?: {
    clientType?: 'public' | 'confidential'
    grantTypes?: string[]
    scopes?: string[]
    redirectUris?: string[]
    requirePAR?: boolean
    /** PKCE が必須か。client metadata `require_pkce` (RFC 9700 / OAuth 2.1 階段化)。
     *  public / FAPI クライアントは true、confidential は明示 false で opt-out 可。 */
    requirePkce?: boolean
    authenticated?: boolean
    roles?: string[]
    disabledAt?: string | null
  }
}

// SCL の permissions セクションに宣言されたアクションが、AuthZEN action 名と対応する。
// AuthZEN action 名は伝統的に "domain:verb" の snake_case 表記を採るため、ここで明示的に
// PascalCase（SCL）→ snake_case（実装）にマッピングする。
export const ACTION_NAMES = {
  TokenGrantAuthorizationCode: 'token:grant_authorization_code',
  TokenGrantRefresh: 'token:grant_refresh',
  TokenGrantClientCredentials: 'token:grant_client_credentials',
  TokenGrantDeviceCode: 'token:grant_device_code',
  TokenIntrospect: 'token:introspect',
  TokenRevoke: 'token:revoke',
  UserInfoRead: 'userinfo:read',
  AuthorizeInitiate: 'authorize:initiate',
  AdminUserRead: 'admin:user_read',
  AdminUserCreate: 'admin:user_create',
  AdminUserUpdate: 'admin:user_update',
} as const

export type ActionName = (typeof ACTION_NAMES)[keyof typeof ACTION_NAMES]

export interface AuthZENAction {
  name: ActionName
}

export interface AuthZENResource {
  type:
    | 'AuthorizationCode'
    | 'RefreshToken'
    | 'AccessToken'
    | 'AuthorizationRequest'
    | 'UserInfo'
    | 'DeviceCode'
    | 'User'
  id?: string
  properties?: {
    issuedToClientId?: string
    redirectUri?: string
    codeChallenge?: string
    codeChallengeMethod?: string
    issuedAt?: string
    expiresAt?: string
    redeemed?: boolean
    revoked?: boolean
    rotated?: boolean
    absoluteExpiresAt?: string
    senderConstraint?: { type: 'dpop' | 'mtls' } | null
    scopes?: string[]
    approved?: boolean
  }
}

export interface AuthZENContext {
  codeVerifier?: string
  redirectUri?: string
  proofOfPossession?: { valid: boolean; jkt?: string; x5tS256?: string } | null
  parUsed?: boolean
  authenticated?: boolean
  now?: string
}

export interface AuthZENRequest {
  subject: AuthZENSubject
  action: AuthZENAction
  resource: AuthZENResource
  context?: AuthZENContext
}

export interface AuthZENResponse {
  decision: 'Permit' | 'Deny'
  reasons?: string[]
}

// ---------------------------------------------------------------
// アクション → 名前付きルール
// ---------------------------------------------------------------
//
// SCL の `permissions.<Name>.allow_when` 内の各述語を、運用上の「名前付きルール」に分解する。
// この分解は Cedar / OPA Rego へのマッピング点であり、SCL の意図と一対一で対応する。

const ACTION_RULES: Record<ActionName, string[]> = {
  'token:grant_authorization_code': [
    'client_must_declare_grant',
    'pkce_verification_passed',
    'redirect_uri_exact_match',
    'code_not_redeemed',
    'code_not_expired',
  ],
  'token:grant_refresh': [
    'client_must_declare_grant',
    'token_active',
    'token_within_absolute_ttl',
    'sender_constraint_satisfied',
  ],
  'token:grant_client_credentials': ['client_is_confidential', 'client_must_declare_grant'],
  'token:grant_device_code': ['device_code_approved', 'device_code_not_expired'],
  'token:introspect': ['caller_is_authenticated_client'],
  'token:revoke': ['caller_owns_token'],
  'userinfo:read': ['token_has_openid_scope', 'token_active'],
  'authorize:initiate': [
    'client_registered',
    'redirect_uri_registered',
    'scope_subset_of_client_scope',
    'pkce_present',
    'par_required_if_fapi',
  ],
  'admin:user_read': ['actor_is_admin', 'actor_is_active', 'actor_is_authenticated'],
  'admin:user_create': ['actor_is_admin', 'actor_is_active', 'actor_is_authenticated'],
  'admin:user_update': ['actor_is_admin', 'actor_is_active', 'actor_is_authenticated'],
}

// ---------------------------------------------------------------
// 評価関数（純粋関数、副作用なし）
// ---------------------------------------------------------------

type RuleEvaluator = (req: AuthZENRequest) => boolean

const ruleEvaluators: Record<string, RuleEvaluator> = {
  client_must_declare_grant(req) {
    const grantType = grantTypeFromAction(req.action.name)
    if (!grantType) return false
    return req.subject.properties?.grantTypes?.includes(grantType) ?? false
  },

  pkce_verification_passed(req) {
    const verifier = req.context?.codeVerifier
    const challenge = req.resource.properties?.codeChallenge
    // challenge 不在は require_pkce=false で発行された code (PKCE 省略合意)。
    // この場合 verifier も来ていなければ OK。来ていれば downgrade とみなし上流で拒否済み。
    if (!challenge) return !verifier
    if (!verifier) return false
    return true
  },

  redirect_uri_exact_match(req) {
    const requested = req.context?.redirectUri
    const stored = req.resource.properties?.redirectUri
    return !!requested && !!stored && requested === stored
  },

  redirect_uri_registered(req) {
    const requested = req.context?.redirectUri
    return req.subject.properties?.redirectUris?.includes(requested ?? '') ?? false
  },

  code_not_redeemed(req) {
    return req.resource.properties?.redeemed !== true
  },

  code_not_expired(req) {
    return !isExpired(req.resource.properties?.expiresAt, req.context?.now)
  },

  token_active(req) {
    const p = req.resource.properties
    return p?.revoked !== true && p?.rotated !== true
  },

  token_within_absolute_ttl(req) {
    return !isExpired(req.resource.properties?.absoluteExpiresAt, req.context?.now)
  },

  sender_constraint_satisfied(req) {
    const c = req.resource.properties?.senderConstraint
    if (!c) return true
    return req.context?.proofOfPossession?.valid === true
  },

  client_is_confidential(req) {
    return req.subject.properties?.clientType === 'confidential'
  },

  device_code_approved(req) {
    return req.resource.properties?.approved === true
  },

  device_code_not_expired(req) {
    return !isExpired(req.resource.properties?.expiresAt, req.context?.now)
  },

  caller_is_authenticated_client(req) {
    return req.subject.type === 'Client' && req.subject.properties?.authenticated === true
  },

  caller_owns_token(req) {
    return req.subject.id === req.resource.properties?.issuedToClientId
  },

  token_has_openid_scope(req) {
    return req.resource.properties?.scopes?.includes('openid') ?? false
  },

  client_registered(req) {
    return req.subject.type === 'Client' && !!req.subject.id
  },

  scope_subset_of_client_scope(req) {
    const requested = req.resource.properties?.scopes ?? []
    const allowed = req.subject.properties?.scopes ?? []
    return requested.every((s) => allowed.includes(s))
  },

  pkce_present(req) {
    // require_pkce が明示 false なら code_challenge 不要 (legacy confidential client)。
    // 未指定 or true なら必須。public / FAPI クライアントの require_pkce は
    // authorize-request use case 側で true に確定させてからこの policy に流す。
    if (req.subject.properties?.requirePkce === false) return true
    return !!req.resource.properties?.codeChallenge
  },

  par_required_if_fapi(req) {
    if (req.subject.properties?.requirePAR !== true) return true
    return req.context?.parUsed === true
  },

  actor_is_admin(req) {
    return req.subject.type === 'User' && (req.subject.properties?.roles?.includes('admin') ?? false)
  },

  actor_is_active(req) {
    return req.subject.type === 'User' && !req.subject.properties?.disabledAt
  },

  actor_is_authenticated(req) {
    return req.subject.type === 'User' && req.context?.authenticated === true
  },
}

function grantTypeFromAction(action: ActionName): string | null {
  switch (action) {
    case 'token:grant_authorization_code':
      return 'authorization_code'
    case 'token:grant_refresh':
      return 'refresh_token'
    case 'token:grant_client_credentials':
      return 'client_credentials'
    case 'token:grant_device_code':
      return 'urn:ietf:params:oauth:grant-type:device_code'
    default:
      return null
  }
}

function isExpired(expiresAt: string | undefined, nowIso: string | undefined): boolean {
  if (!expiresAt) return false
  const exp = Date.parse(expiresAt)
  const now = nowIso ? Date.parse(nowIso) : Date.now()
  return now >= exp
}

export function evaluate(req: AuthZENRequest): AuthZENResponse {
  const rules = ACTION_RULES[req.action.name]
  if (!rules) {
    return { decision: 'Deny', reasons: [`未定義のアクション: ${req.action.name}`] }
  }

  const failed: string[] = []
  for (const ruleId of rules) {
    const evaluator = ruleEvaluators[ruleId]
    if (!evaluator) {
      failed.push(`未実装のルール: ${ruleId}`)
      continue
    }
    if (!evaluator(req)) {
      failed.push(ruleId)
    }
  }

  if (failed.length > 0) return { decision: 'Deny', reasons: failed }
  return { decision: 'Permit' }
}

// ---------------------------------------------------------------
// 整合性検査用エクスポート
// ---------------------------------------------------------------

export const ALL_RULE_IDS: string[] = Object.values(ACTION_RULES).flat()
export const IMPLEMENTED_RULE_IDS: string[] = Object.keys(ruleEvaluators)

/** SCL の `permissions` に宣言された全アクション（PascalCase）が AuthZEN action 名にマップされているか */
export function sclPermissionsCoveredByActionNames(): { missing: string[]; extra: string[] } {
  const sclActions = Object.keys(scl.permissions)
  const mapped = Object.keys(ACTION_NAMES)
  return {
    missing: sclActions.filter((a) => !mapped.includes(a)),
    extra: mapped.filter((a) => !sclActions.includes(a)),
  }
}
