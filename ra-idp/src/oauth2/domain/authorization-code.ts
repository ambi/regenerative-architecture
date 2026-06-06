/**
 * Layer 3 — Application Logic
 *
 * 認可コードのドメインモデル。
 * RFC 9700 §4.10 に従う「単一使用」と TTL 制約を含む。
 */

import { randomBytes } from 'crypto'
import { AuthorizationCodeSchema, type AuthorizationCode } from '../../spec-bindings/schemas'
import { OAuthError } from '../protocol/oauth-error'

export type { AuthorizationCode }

const CODE_BYTES = 32 // 256 ビット

export function generateAuthorizationCode(input: {
  authorization_request_id: string
  client_id: string
  sub: string
  scopes: string[]
  redirect_uri: string
  code_challenge: string
  code_challenge_method: 'S256'
  nonce?: string
  auth_time: number
  ttl_seconds?: number
  now?: Date
}): AuthorizationCode {
  const ttl = input.ttl_seconds ?? 60
  const now = input.now ?? new Date()
  return AuthorizationCodeSchema.parse({
    code: randomBytes(CODE_BYTES).toString('base64url'),
    authorization_request_id: input.authorization_request_id,
    client_id: input.client_id,
    sub: input.sub,
    scopes: input.scopes,
    redirect_uri: input.redirect_uri,
    code_challenge: input.code_challenge,
    code_challenge_method: input.code_challenge_method,
    nonce: input.nonce,
    auth_time: input.auth_time,
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + ttl * 1000).toISOString(),
  })
}

export function isExpired(code: AuthorizationCode, now: Date = new Date()): boolean {
  return now.getTime() >= Date.parse(code.expires_at)
}

export function isRedeemed(code: AuthorizationCode): boolean {
  return !!code.redeemed_at
}

export function markRedeemed(code: AuthorizationCode, now: Date = new Date()): AuthorizationCode {
  if (isRedeemed(code)) {
    throw new OAuthError('invalid_grant', '認可コードはすでに使用されています')
  }
  return AuthorizationCodeSchema.parse({ ...code, redeemed_at: now.toISOString() })
}
