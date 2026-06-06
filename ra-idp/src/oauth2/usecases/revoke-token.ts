/**
 * Layer 3 — Application Logic
 *
 * /revoke (RFC 7009)。 リフレッシュトークンを失効する。
 * 本アプリではアクセストークンの失効リストは持たず（短寿命 JWT のため）、
 * リフレッシュトークン失効と family revoke を実装する。
 */

import { hashToken } from '../domain/refresh-token'
import type { RefreshTokenStore } from '../ports/refresh-token-store'

export async function revokeTokenUseCase(
  deps: { refreshStore: RefreshTokenStore },
  token: string,
  emit: (event: { type: string; [k: string]: unknown }) => void,
  now: Date = new Date(),
): Promise<void> {
  const hash = hashToken(token)
  const record = await deps.refreshStore.findByHash(hash)
  if (!record) {
    // RFC 7009 §2.2: 未知のトークンに対しても 200 OK を返すため、no-op
    return
  }
  await deps.refreshStore.revokeFamily(record.family_id)
  emit({
    type: 'TokenRevoked',
    occurredAt: now.toISOString(),
    tokenType: 'refresh_token',
    tokenId: record.id,
    reason: 'client_initiated',
  })
}
