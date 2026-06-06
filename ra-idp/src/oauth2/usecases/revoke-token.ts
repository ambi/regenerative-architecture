/**
 * Layer 3 — Application Logic
 *
 * /revoke (RFC 7009)。
 *
 * - refresh_token: family revoke (RFC 9700 §4.10) を実行
 * - access_token (JWT): jti を denylist に積み、/introspect / /userinfo 経路で
 *   active=false 扱いにする (Phase 1 — JWT 即時失効)
 *
 * RFC 7009 §2.2 に従い、未知のトークンに対しても 200 OK を返す (no-op)。
 */

import { hashToken } from '../domain/refresh-token'
import type { AccessTokenDenylist } from '../ports/access-token-denylist'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { TokenIntrospector } from '../ports/token-introspector'

export async function revokeTokenUseCase(
  deps: {
    refreshStore: RefreshTokenStore
    /** JWT access token を denylist に追加するための introspector。 */
    introspector?: TokenIntrospector
    accessTokenDenylist?: AccessTokenDenylist
  },
  token: string,
  emit: (event: { type: string; [k: string]: unknown }) => void,
  now: Date = new Date(),
): Promise<void> {
  // 1) refresh_token として試す (opaque)
  const hash = hashToken(token)
  const record = await deps.refreshStore.findByHash(hash)
  if (record) {
    await deps.refreshStore.revokeFamily(record.family_id)
    emit({
      type: 'TokenRevoked',
      occurredAt: now.toISOString(),
      tokenType: 'refresh_token',
      tokenId: record.id,
      reason: 'client_initiated',
    })
    return
  }

  // 2) access_token (JWT) として試す。invalid JWT は no-op で 200 を返す (RFC 7009 §2.2)
  if (deps.introspector && deps.accessTokenDenylist && looksLikeJwt(token)) {
    const res = await deps.introspector.introspectAccessToken(token).catch(() => null)
    if (res && res.active && res.jti && res.exp) {
      await deps.accessTokenDenylist.add(res.jti, new Date(res.exp * 1000))
      emit({
        type: 'TokenRevoked',
        occurredAt: now.toISOString(),
        tokenType: 'access_token',
        tokenId: res.jti,
        reason: 'client_initiated',
      })
    }
  }
}

function looksLikeJwt(token: string): boolean {
  return token.split('.').length === 3
}
