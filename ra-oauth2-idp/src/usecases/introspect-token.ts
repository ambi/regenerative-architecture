/**
 * Layer 3 — Application Logic
 *
 * /introspect (RFC 7662) の中核ロジック。
 *
 * 本アプリではアクセストークンが JWT のため、JWT 検証 + リボケーション確認のみを行う。
 * リフレッシュトークンの introspect も同じエンドポイントで処理する（ストア突合）。
 */

import { hashToken } from '../domain/refresh-token'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { TokenIntrospector } from '../ports/token-introspector'

export interface IntrospectInput {
  token: string
  token_type_hint?: 'access_token' | 'refresh_token'
}

export interface IntrospectionResponse {
  active: boolean
  scope?: string
  client_id?: string
  username?: string
  token_type?: string
  exp?: number
  iat?: number
  nbf?: number
  sub?: string
  aud?: string | string[]
  iss?: string
  jti?: string
  cnf?: { jkt?: string; 'x5t#S256'?: string }
}

export async function introspectTokenUseCase(
  deps: {
    introspector: TokenIntrospector
    refreshStore: RefreshTokenStore
  },
  input: IntrospectInput,
  now: Date = new Date(),
): Promise<IntrospectionResponse> {
  // 最初に refresh_token として試す（hint があれば優先、なければ両方試す）
  if (input.token_type_hint === 'refresh_token' || input.token_type_hint === undefined) {
    const hash = hashToken(input.token)
    const record = await deps.refreshStore.findByHash(hash)
    if (record) {
      const active =
        !record.revoked && !record.rotated && now.getTime() < Date.parse(record.absolute_expires_at)
      if (!active) return { active: false }
      return {
        active: true,
        scope: record.scopes.join(' '),
        client_id: record.client_id,
        sub: record.sub,
        token_type: 'refresh_token',
        iat: Math.floor(Date.parse(record.issued_at) / 1000),
        exp: Math.floor(Date.parse(record.expires_at) / 1000),
        jti: record.id,
        cnf: record.sender_constraint
          ? {
              ...(record.sender_constraint.jkt ? { jkt: record.sender_constraint.jkt } : {}),
              ...(record.sender_constraint['x5t#S256']
                ? { 'x5t#S256': record.sender_constraint['x5t#S256'] }
                : {}),
            }
          : undefined,
      }
    }
  }

  // access_token として検証
  return deps.introspector.introspectAccessToken(input.token)
}
