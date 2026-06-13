/**
 * Layer 3 — Application Logic
 *
 * リフレッシュトークンによる再発行。
 * ADR-004 のローテーション + ファミリー失効を実装する。
 */

import { hashToken, isAbsoluteExpired, isReplay, rotate } from '../domain/refresh-token'
import { OAuthError } from '../protocol/oauth-error'
import { evaluate } from '../../spec-bindings/policy/client-authorization'
import type { ClientRepository } from '../ports/client-repository'
import type { UserRepository } from '../../authentication/ports/user-repository'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { TokenIssuer } from '../ports/token-issuer'

export interface RefreshInput {
  client_id: string
  refresh_token: string
  /** DPoP / mTLS バインドがある場合の所有証明。 */
  proof_jkt?: string
  proof_x5t_s256?: string
}

export interface RefreshResult {
  access_token: string
  refresh_token: string
  token_type: 'Bearer' | 'DPoP'
  expires_in: number
  scope: string
}

export async function refreshTokenUseCase(
  deps: {
    clientRepo: ClientRepository
    userRepo: UserRepository
    refreshStore: RefreshTokenStore
    tokenIssuer: TokenIssuer
  },
  input: RefreshInput,
  emit: (event: { type: string; [k: string]: unknown }) => void,
  now: Date = new Date(),
): Promise<RefreshResult> {
  const client = await deps.clientRepo.findById(input.client_id)
  if (!client) {
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }

  const hash = hashToken(input.refresh_token)
  const record = await deps.refreshStore.findByHash(hash)
  if (!record) {
    throw new OAuthError('invalid_grant', 'リフレッシュトークンが無効です')
  }

  if (record.client_id !== client.client_id) {
    // 他人のリフレッシュトークンの使用 → 即座にファミリー失効
    await deps.refreshStore.revokeFamily(record.family_id)
    emit({
      type: 'RefreshTokenReuseDetected',
      occurredAt: now.toISOString(),
      familyId: record.family_id,
      tokenId: record.id,
      clientId: client.client_id,
    })
    throw new OAuthError('invalid_grant', 'リフレッシュトークンの所有者が一致しません')
  }

  if (isReplay(record)) {
    // ローテーション済み or 失効済み → ファミリー全失効 + アラート
    await deps.refreshStore.revokeFamily(record.family_id)
    emit({
      type: 'RefreshTokenReuseDetected',
      occurredAt: now.toISOString(),
      familyId: record.family_id,
      tokenId: record.id,
      clientId: client.client_id,
    })
    throw new OAuthError('invalid_grant', 'リフレッシュトークンはすでに使用されています')
  }

  if (isAbsoluteExpired(record, now)) {
    throw new OAuthError('invalid_grant', 'リフレッシュトークンが絶対期限切れです')
  }

  // ポリシー評価
  const decision = evaluate({
    subject: {
      type: 'Client',
      id: client.client_id,
      properties: { grantTypes: client.grant_types },
    },
    action: { name: 'token:grant_refresh' },
    resource: {
      type: 'RefreshToken',
      properties: {
        revoked: record.revoked,
        rotated: record.rotated,
        absoluteExpiresAt: record.absolute_expires_at,
        senderConstraint: record.sender_constraint ?? null,
      },
    },
    context: {
      proofOfPossession: record.sender_constraint
        ? {
            valid:
              (record.sender_constraint.type === 'dpop' &&
                record.sender_constraint.jkt === input.proof_jkt) ||
              (record.sender_constraint.type === 'mtls' &&
                record.sender_constraint['x5t#S256'] === input.proof_x5t_s256),
          }
        : null,
      now: now.toISOString(),
    },
  })
  if (decision.decision === 'Deny') {
    throw new OAuthError(
      'invalid_grant',
      `リフレッシュが拒否されました: ${decision.reasons?.join(', ')}`,
    )
  }

  // ADR-031: 無効化された user のトークン再発行はローテーション前に拒否する。
  // 状態を壊さないよう、refresh-store rotate より前に確認する。
  const user = await deps.userRepo.findBySub(record.sub)
  if (!user) throw new OAuthError('server_error', 'ユーザーが存在しません')
  if (user.disabled_at) {
    throw new OAuthError('invalid_grant', 'ユーザーは無効化されています')
  }

  // ローテーション: 新トークン発行 + 旧トークンを rotated にマーク
  const { token: newToken, record: newRecord } = rotate(record, now)
  const rotated = await deps.refreshStore.rotate(record.id, newRecord)
  if (!rotated) {
    // 並行ローテーション失敗 → ファミリー失効
    await deps.refreshStore.revokeFamily(record.family_id)
    throw new OAuthError('invalid_grant', '並行リフレッシュにより失効しました')
  }

  emit({
    type: 'RefreshTokenRotated',
    occurredAt: now.toISOString(),
    oldTokenId: record.id,
    newTokenId: newRecord.id,
    familyId: record.family_id,
  })

  const stored = record.sender_constraint
  const senderConstraint:
    | { type: 'dpop'; jkt: string }
    | { type: 'mtls'; 'x5t#S256': string }
    | null =
    stored && stored.type === 'dpop' && stored.jkt
      ? { type: 'dpop', jkt: stored.jkt }
      : stored && stored.type === 'mtls' && stored['x5t#S256']
        ? { type: 'mtls', 'x5t#S256': stored['x5t#S256'] }
        : null
  const { token: access_token } = await deps.tokenIssuer.signAccessToken({
    client,
    sub: record.sub,
    scopes: record.scopes,
    senderConstraint,
    authTime: Math.floor(now.getTime() / 1000),
  })

  return {
    access_token,
    refresh_token: newToken,
    token_type: senderConstraint ? 'DPoP' : 'Bearer',
    expires_in: deps.tokenIssuer.getAccessTokenTtlSeconds(),
    scope: record.scopes.join(' '),
  }
}
