/**
 * Layer 3 — Specification Binding (TypeScript)（grant matrix）
 *
 * 仕様本体は spec/scl.yaml の `models`（enum）と `permissions`。
 * このファイルは TypeScript の narrow 型と検証関数を提供するバインディング。
 *
 * 各定数 (SUPPORTED_GRANT_TYPES 等) は `as const` の narrow タプルで宣言し、
 * SCL の enum 値との整合は ../invariants.test.ts で検証する。
 */

import { scl, enumWireValues, toWire } from '../scl'

// ===============================================================
// Enum 値（ワイヤ形式、narrow タプル）
// ===============================================================

export const SUPPORTED_GRANT_TYPES = [
  'authorization_code',
  'refresh_token',
  'client_credentials',
  'urn:ietf:params:oauth:grant-type:device_code',
] as const
export type GrantType = (typeof SUPPORTED_GRANT_TYPES)[number]

export const CLIENT_TYPES = ['public', 'confidential'] as const
export type ClientType = (typeof CLIENT_TYPES)[number]

export const TOKEN_AUTH_METHODS = [
  'client_secret_basic',
  'client_secret_post',
  'private_key_jwt',
  'tls_client_auth',
  'none',
] as const
export type TokenAuthMethod = (typeof TOKEN_AUTH_METHODS)[number]

export const RESPONSE_TYPES = ['code'] as const
export type ResponseType = (typeof RESPONSE_TYPES)[number]

export const RESPONSE_MODES = ['query', 'form_post'] as const
export type ResponseMode = (typeof RESPONSE_MODES)[number]

export const SIGNATURE_ALGORITHMS = ['PS256', 'ES256'] as const
export type SignatureAlgorithm = (typeof SIGNATURE_ALGORITHMS)[number]

// ===============================================================
// grant matrix（標準 OAuth 2.0 / OIDC RFC からの写し）
// ===============================================================

interface GrantSpecEntry {
  allowed_client_types: ClientType[]
  requires_pkce: boolean
  response_types: ResponseType[]
  issues: string[]
}

const GRANT_SPEC: Record<GrantType, GrantSpecEntry> = {
  authorization_code: {
    allowed_client_types: ['public', 'confidential'],
    requires_pkce: true,
    response_types: ['code'],
    issues: ['access_token', 'refresh_token', 'id_token'],
  },
  refresh_token: {
    allowed_client_types: ['public', 'confidential'],
    requires_pkce: false,
    response_types: [],
    issues: ['access_token', 'refresh_token'],
  },
  client_credentials: {
    allowed_client_types: ['confidential'],
    requires_pkce: false,
    response_types: [],
    issues: ['access_token'],
  },
  'urn:ietf:params:oauth:grant-type:device_code': {
    allowed_client_types: ['public', 'confidential'],
    requires_pkce: false,
    response_types: [],
    issues: ['access_token', 'refresh_token', 'id_token'],
  },
}

export function getGrantSpec(grant: GrantType): GrantSpecEntry {
  return GRANT_SPEC[grant]
}

export function grantAllowsClientType(grant: GrantType, clientType: ClientType): boolean {
  return GRANT_SPEC[grant].allowed_client_types.includes(clientType)
}

export function grantRequiresPkce(grant: GrantType): boolean {
  return GRANT_SPEC[grant].requires_pkce
}

// ===============================================================
// SCL 整合性検査用ビュー
// ===============================================================

/** SCL の各 enum モデルからワイヤ形式の値リストを取得（テストが期待値として使う） */
export function sclEnumWire(modelName: string): string[] {
  return enumWireValues(modelName)
}

/** Discovery を組み立てるときに使う、SCL annotations.discovery_template 由来の値 */
export const DISCOVERY_TEMPLATE = (scl.annotations?.discovery_template ?? {}) as {
  scopes_supported: string[]
  subject_types_supported: string[]
  claims_supported: string[]
  ui_locales_supported: string[]
  introspection_endpoint_auth_methods: string[]
  revocation_endpoint_auth_methods: string[]
}

export function discoveryIntrospectionAuthMethods(): string[] {
  return (DISCOVERY_TEMPLATE.introspection_endpoint_auth_methods ?? []).map(toWire)
}

export function discoveryRevocationAuthMethods(): string[] {
  return (DISCOVERY_TEMPLATE.revocation_endpoint_auth_methods ?? []).map(toWire)
}
