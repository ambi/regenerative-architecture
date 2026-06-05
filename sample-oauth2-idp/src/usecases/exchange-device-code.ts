/**
 * Layer 3 — Application Logic
 *
 * /token の device_code グラント (RFC 8628 §3.4 / §3.5)。
 *
 * クライアントはこのエンドポイントをポーリングする。状態に応じて:
 *   issued / user_code_entered / authorization_pending → authorization_pending
 *   (interval より速いポーリング)                        → slow_down
 *   denied                                              → access_denied
 *   expired / 期限切れ                                   → expired_token
 *   exchanged                                           → invalid_grant (使用済み)
 *   approved                                            → トークン発行 + exchanged
 *
 * 承認済みからの発行は exchange-code-for-token と同じく
 * access_token + refresh_token + id_token(openid 時) を返す。
 */

import { OAuthError } from '../domain/errors'
import { hashDeviceCode, isDeviceExpired } from '../domain/device-authorization'
import { generateInitial } from '../domain/refresh-token'
import { transitionDeviceCode, DEVICE_CODE_POLLING } from '../spec-bindings/flows/flows'
import type { DeviceAuthorization } from '../spec-bindings/schemas'
import type { ClientRepository } from '../ports/client-repository'
import type { UserRepository } from '../ports/user-repository'
import type { DeviceCodeStore } from '../ports/device-code-store'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { TokenIssuer } from '../ports/token-issuer'

export interface ExchangeDeviceCodeInput {
  client_id: string
  device_code: string
  dpop_jkt?: string
}

export interface ExchangeDeviceCodeResult {
  response: {
    access_token: string
    token_type: 'Bearer' | 'DPoP'
    expires_in: number
    refresh_token: string
    id_token?: string
    scope: string
  }
  audit: {
    sub: string
    jti: string
    scopes: string[]
    senderConstraint: 'none' | 'dpop' | 'mtls'
    refreshTokenId: string
    refreshFamilyId: string
  }
}

export async function exchangeDeviceCodeUseCase(
  deps: {
    clientRepo: ClientRepository
    userRepo: UserRepository
    deviceCodeStore: DeviceCodeStore
    refreshStore: RefreshTokenStore
    tokenIssuer: TokenIssuer
  },
  input: ExchangeDeviceCodeInput,
  now: Date = new Date(),
): Promise<ExchangeDeviceCodeResult> {
  const client = await deps.clientRepo.findById(input.client_id)
  if (!client) {
    throw new OAuthError('invalid_client', 'クライアント認証に失敗しました')
  }

  const rec = await deps.deviceCodeStore.findByDeviceCodeHash(hashDeviceCode(input.device_code))
  if (!rec) {
    throw new OAuthError('invalid_grant', 'device_code が無効です')
  }
  if (rec.client_id !== client.client_id) {
    throw new OAuthError('invalid_grant', 'device_code がクライアントと一致しません')
  }
  if (isDeviceExpired(rec, now)) {
    throw new OAuthError('expired_token', 'device_code の有効期限が切れています')
  }

  // ポーリング間隔の強制 (RFC 8628 §3.5)。interval より速いと slow_down。
  // 注: 単一レコードの read-modify-write。複数レプリカ間で厳密ではないが、
  // slow_down はソフトシグナルであり、過剰要求の抑制が目的なので許容する。
  if (rec.last_polled_at) {
    const elapsed = (now.getTime() - Date.parse(rec.last_polled_at)) / 1000
    if (elapsed < rec.interval_seconds) {
      const slowed: DeviceAuthorization = {
        ...rec,
        last_polled_at: now.toISOString(),
        interval_seconds: rec.interval_seconds + DEVICE_CODE_POLLING.slow_down_increment_seconds,
      }
      await deps.deviceCodeStore.update(slowed)
      throw new OAuthError('slow_down', 'ポーリングが速すぎます。interval を増やしてください')
    }
  }
  await deps.deviceCodeStore.update({ ...rec, last_polled_at: now.toISOString() })

  switch (rec.state) {
    case 'denied':
      throw new OAuthError('access_denied', 'ユーザーが認可を拒否しました')
    case 'exchanged':
      throw new OAuthError('invalid_grant', 'device_code はすでに使用済みです')
    case 'expired':
      throw new OAuthError('expired_token', 'device_code の有効期限が切れています')
    case 'issued':
    case 'user_code_entered':
    case 'authorization_pending':
      throw new OAuthError('authorization_pending', 'ユーザーの承認待ちです')
    case 'approved':
      break
    default:
      throw new OAuthError('invalid_grant', `不正な device_code 状態: ${rec.state}`)
  }

  if (!rec.sub) {
    throw new OAuthError('server_error', '承認済みだが sub が記録されていません')
  }

  // approved → exchanged に進めてから発行する (二重発行防止)。
  const exchangedState = transitionDeviceCode('approved', 'exchange')
  if (!exchangedState) {
    throw new OAuthError('server_error', 'device_code の状態遷移に失敗しました')
  }

  const user = await deps.userRepo.findBySub(rec.sub)
  if (!user) {
    throw new OAuthError('server_error', 'ユーザーが存在しません')
  }

  const senderConstraint = input.dpop_jkt ? { type: 'dpop' as const, jkt: input.dpop_jkt } : null

  const { token: access_token, jti } = await deps.tokenIssuer.signAccessToken({
    client,
    sub: rec.sub,
    scopes: rec.scopes,
    senderConstraint,
    authTime: rec.auth_time ?? Math.floor(now.getTime() / 1000),
  })

  const { token: refresh_token, record: refreshRecord } = generateInitial({
    client_id: client.client_id,
    sub: rec.sub,
    scopes: rec.scopes,
    sender_constraint: senderConstraint,
    now,
  })
  await deps.refreshStore.save(refreshRecord)

  await deps.deviceCodeStore.update({
    ...rec,
    state: exchangedState,
    last_polled_at: now.toISOString(),
    issued_family_id: refreshRecord.family_id,
  })

  let id_token: string | undefined
  if (rec.scopes.includes('openid')) {
    id_token = await deps.tokenIssuer.signIdToken({
      client,
      user,
      scopes: rec.scopes,
      authTime: rec.auth_time ?? Math.floor(now.getTime() / 1000),
      atHashFor: access_token,
    })
  }

  return {
    response: {
      access_token,
      token_type: senderConstraint ? 'DPoP' : 'Bearer',
      expires_in: deps.tokenIssuer.getAccessTokenTtlSeconds(),
      refresh_token,
      id_token,
      scope: rec.scopes.join(' '),
    },
    audit: {
      sub: rec.sub,
      jti,
      scopes: rec.scopes,
      senderConstraint: senderConstraint ? 'dpop' : 'none',
      refreshTokenId: refreshRecord.id,
      refreshFamilyId: refreshRecord.family_id,
    },
  }
}
