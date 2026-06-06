/**
 * Layer 3 — Specification Binding (TypeScript / Zod)
 *
 * 仕様本体（language-agnostic）は spec/scl.yaml の `models` セクション。
 * `bun run gen:scl` で gen/*.json に JSON Schema として派生する。
 * 本ファイルは TypeScript 実装からランタイム検証するための Zod バインディング。
 * 別言語に移植する場合は spec/scl.yaml + gen/*.json を直接消費し、本ディレクトリを該当言語版で置き換える。
 */

import { z } from 'zod'
import {
  SUPPORTED_GRANT_TYPES,
  CLIENT_TYPES,
  TOKEN_AUTH_METHODS,
  RESPONSE_TYPES,
} from './grants/grant-types'

// ===============================================================
// クライアント
// ===============================================================

export const ClientSchema = z.object({
  client_id: z.string().min(1).max(128),
  client_secret_hash: z.string().optional(),
  client_name: z.string().min(1).max(200).optional(),
  client_type: z.enum(CLIENT_TYPES),
  redirect_uris: z.array(z.string().url()).min(1),
  grant_types: z.array(z.enum(SUPPORTED_GRANT_TYPES)).min(1),
  response_types: z.array(z.enum(RESPONSE_TYPES)).default([]),
  token_endpoint_auth_method: z.enum(TOKEN_AUTH_METHODS),
  scope: z.string(),
  jwks_uri: z.string().url().optional(),
  jwks: z.record(z.unknown()).optional(),
  tls_client_auth_subject_dn: z.string().optional(),
  id_token_signed_response_alg: z.enum(['PS256', 'ES256']).default('PS256'),
  require_pushed_authorization_requests: z.boolean().default(false),
  dpop_bound_access_tokens: z.boolean().default(false),
  fapi_profile: z.enum(['none', 'fapi_2_security_profile']).default('none'),
  created_at: z.string().datetime(),
})
export type Client = z.infer<typeof ClientSchema>

// ===============================================================
// ユーザー
// ===============================================================

export const UserSchema = z.object({
  sub: z.string().min(1),
  preferred_username: z.string().min(1).max(100),
  password_hash: z.string(),
  name: z.string().optional(),
  given_name: z.string().optional(),
  family_name: z.string().optional(),
  email: z.string().email().optional(),
  email_verified: z.boolean().default(false),
  mfa_enrolled: z.boolean().default(false),
  created_at: z.string().datetime(),
  updated_at: z.string().datetime(),
  deleted_at: z.string().datetime().optional(),
})
export type User = z.infer<typeof UserSchema>

// ===============================================================
// コンセント
// ===============================================================

export const ConsentSchema = z.object({
  sub: z.string(),
  client_id: z.string(),
  scopes: z.array(z.string()).min(1),
  granted_at: z.string().datetime(),
  expires_at: z.string().datetime(),
  revoked_at: z.string().datetime().optional(),
})
export type Consent = z.infer<typeof ConsentSchema>

// ===============================================================
// 認可リクエスト（フロー追跡用、サーバー内部の中間ステート）
// ===============================================================

export const AuthorizationRequestSchema = z.object({
  id: z.string().uuid(),
  state: z.enum([
    'received',
    'authentication_pending',
    'authenticated',
    'consent_pending',
    'consented',
    'code_issued',
    'exchanged',
    'rejected',
    'expired',
  ]),
  client_id: z.string(),
  redirect_uri: z.string().url(),
  response_type: z.literal('code'),
  scope: z.string(),
  state_param: z.string().optional(),
  nonce: z.string().optional(),
  code_challenge: z.string(),
  code_challenge_method: z.literal('S256'),
  prompt: z.string().optional(),
  max_age: z.number().int().nonnegative().optional(),
  id_token_hint: z.string().optional(),
  par_request_uri: z.string().optional(),
  sub: z.string().optional(),
  auth_time: z.number().int().optional(),
  created_at: z.string().datetime(),
  expires_at: z.string().datetime(),
})
export type AuthorizationRequest = z.infer<typeof AuthorizationRequestSchema>

// ===============================================================
// 認可コード
// ===============================================================

export const AuthorizationCodeSchema = z.object({
  code: z.string(),
  authorization_request_id: z.string().uuid(),
  client_id: z.string(),
  sub: z.string(),
  scopes: z.array(z.string()),
  redirect_uri: z.string().url(),
  code_challenge: z.string(),
  code_challenge_method: z.literal('S256'),
  nonce: z.string().optional(),
  auth_time: z.number().int(),
  issued_at: z.string().datetime(),
  expires_at: z.string().datetime(),
  redeemed_at: z.string().datetime().optional(),
  // 成功した交換で発行された refresh token のファミリー ID。
  // 不正リプレイ検知時に当該ファミリーを失効させるための逆引きインデックス。
  issued_family_id: z.string().uuid().optional(),
})
export type AuthorizationCode = z.infer<typeof AuthorizationCodeSchema>

// ===============================================================
// リフレッシュトークン（ストアレコード）
// ===============================================================

export const RefreshTokenRecordSchema = z.object({
  id: z.string().uuid(),
  hash: z.string(),
  family_id: z.string().uuid(),
  parent_id: z.string().uuid().optional(),
  client_id: z.string(),
  sub: z.string(),
  scopes: z.array(z.string()),
  issued_at: z.string().datetime(),
  expires_at: z.string().datetime(),
  absolute_expires_at: z.string().datetime(),
  revoked: z.boolean().default(false),
  rotated: z.boolean().default(false),
  sender_constraint: z
    .object({
      type: z.enum(['dpop', 'mtls']),
      jkt: z.string().optional(),
      'x5t#S256': z.string().optional(),
    })
    .nullable()
    .optional(),
})
export type RefreshTokenRecord = z.infer<typeof RefreshTokenRecordSchema>

// ===============================================================
// PAR レコード
// ===============================================================

export const PARRecordSchema = z.object({
  request_uri: z.string(),
  client_id: z.string(),
  parameters: z.record(z.string()),
  issued_at: z.string().datetime(),
  expires_at: z.string().datetime(),
  used: z.boolean().default(false),
})
export type PARRecord = z.infer<typeof PARRecordSchema>

// ===============================================================
// デバイス認可 (RFC 8628) — volatile state
// ===============================================================
// 状態機械の本体は spec/scl.yaml state_machines.DeviceCodeFlow。ここはストアレコードの形。

export const DeviceAuthorizationSchema = z.object({
  // device_code はベアラ秘密。プレーンテキストは保存せず SHA-256 ハッシュのみ持つ。
  device_code_hash: z.string(),
  // user_code は人間が verification_uri で入力する短いコード。索引キーになる。
  user_code: z.string(),
  client_id: z.string(),
  scopes: z.array(z.string()),
  state: z.enum([
    'issued',
    'user_code_entered',
    'authorization_pending',
    'approved',
    'denied',
    'exchanged',
    'expired',
  ]),
  sub: z.string().optional(),
  auth_time: z.number().int().optional(),
  interval_seconds: z.number().int().positive(),
  last_polled_at: z.string().datetime().optional(),
  // 成功交換で発行した refresh family（リプレイ失効の逆引き、authorization_code と同様）。
  issued_family_id: z.string().uuid().optional(),
  issued_at: z.string().datetime(),
  expires_at: z.string().datetime(),
})
export type DeviceAuthorization = z.infer<typeof DeviceAuthorizationSchema>

// ===============================================================
// アクセストークン JWT クレーム
// ===============================================================

export const AccessTokenClaimsSchema = z.object({
  iss: z.string().url(),
  sub: z.string(),
  aud: z.union([z.string(), z.array(z.string()).min(1)]),
  client_id: z.string(),
  scope: z.string(),
  exp: z.number().int(),
  iat: z.number().int(),
  nbf: z.number().int().optional(),
  jti: z.string(),
  auth_time: z.number().int().optional(),
  acr: z.string().optional(),
  amr: z.array(z.string()).optional(),
  cnf: z
    .object({
      jkt: z.string().optional(),
      'x5t#S256': z.string().optional(),
    })
    .optional(),
})
export type AccessTokenClaims = z.infer<typeof AccessTokenClaimsSchema>

// ===============================================================
// ID トークン クレーム
// ===============================================================

export const IdTokenClaimsSchema = z.object({
  iss: z.string().url(),
  sub: z.string(),
  aud: z.union([z.string(), z.array(z.string()).min(1)]),
  exp: z.number().int(),
  iat: z.number().int(),
  auth_time: z.number().int(),
  nonce: z.string().optional(),
  acr: z.string().optional(),
  amr: z.array(z.string()).optional(),
  azp: z.string().optional(),
  at_hash: z.string().optional(),
  name: z.string().optional(),
  preferred_username: z.string().optional(),
  email: z.string().email().optional(),
  email_verified: z.boolean().optional(),
})
export type IdTokenClaims = z.infer<typeof IdTokenClaimsSchema>

// ===============================================================
// ドメインイベント（discriminated union）
// ===============================================================

const isoDate = z.string().datetime()

export const DomainEventSchema = z.discriminatedUnion('type', [
  z.object({
    type: z.literal('ClientRegistered'),
    occurredAt: isoDate,
    clientId: z.string(),
    clientType: z.string().optional(),
  }),
  z.object({
    type: z.literal('UserAuthenticated'),
    occurredAt: isoDate,
    sub: z.string(),
    amr: z.array(z.string()).optional(),
  }),
  z.object({
    type: z.literal('AuthenticationFailed'),
    occurredAt: isoDate,
    username: z.string().optional(),
    reason: z.string().optional(),
  }),
  z.object({
    type: z.literal('ConsentGranted'),
    occurredAt: isoDate,
    sub: z.string(),
    clientId: z.string(),
    scopes: z.array(z.string()),
  }),
  z.object({
    type: z.literal('ConsentRevoked'),
    occurredAt: isoDate,
    sub: z.string(),
    clientId: z.string(),
  }),
  z.object({
    type: z.literal('AuthorizationCodeIssued'),
    occurredAt: isoDate,
    clientId: z.string(),
    sub: z.string(),
    scopes: z.array(z.string()),
    codeChallengeMethod: z.string().optional(),
  }),
  z.object({
    type: z.literal('AuthorizationCodeRedeemed'),
    occurredAt: isoDate,
    clientId: z.string(),
    sub: z.string(),
  }),
  z.object({
    type: z.literal('AccessTokenIssued'),
    occurredAt: isoDate,
    jti: z.string(),
    clientId: z.string(),
    sub: z.string(),
    scopes: z.array(z.string()),
    senderConstraint: z.enum(['none', 'dpop', 'mtls']).default('none'),
  }),
  z.object({
    type: z.literal('RefreshTokenIssued'),
    occurredAt: isoDate,
    tokenId: z.string(),
    familyId: z.string(),
    parentId: z.string().optional(),
    clientId: z.string(),
    sub: z.string(),
  }),
  z.object({
    type: z.literal('RefreshTokenRotated'),
    occurredAt: isoDate,
    oldTokenId: z.string(),
    newTokenId: z.string(),
    familyId: z.string(),
  }),
  z.object({
    type: z.literal('TokenRevoked'),
    occurredAt: isoDate,
    tokenType: z.enum(['access_token', 'refresh_token']),
    tokenId: z.string(),
    reason: z.string().optional(),
  }),
  z.object({
    type: z.literal('TokenIntrospected'),
    occurredAt: isoDate,
    rsClientId: z.string(),
    tokenId: z.string(),
    active: z.boolean(),
  }),
  z.object({
    type: z.literal('RefreshTokenReuseDetected'),
    occurredAt: isoDate,
    familyId: z.string(),
    tokenId: z.string(),
    clientId: z.string().optional(),
  }),
  z.object({
    type: z.literal('SigningKeyRotated'),
    occurredAt: isoDate,
    newKid: z.string(),
    previousKid: z.string().optional(),
  }),
  z.object({
    type: z.literal('PARStored'),
    occurredAt: isoDate,
    requestUri: z.string(),
    clientId: z.string(),
  }),
  z.object({
    type: z.literal('DeviceAuthorizationRequested'),
    occurredAt: isoDate,
    clientId: z.string(),
    scopes: z.array(z.string()),
  }),
  z.object({
    type: z.literal('DeviceAuthorizationApproved'),
    occurredAt: isoDate,
    clientId: z.string(),
    sub: z.string(),
  }),
  z.object({
    type: z.literal('DeviceAuthorizationDenied'),
    occurredAt: isoDate,
    clientId: z.string(),
    sub: z.string().optional(),
  }),
])
export type DomainEvent = z.infer<typeof DomainEventSchema>
