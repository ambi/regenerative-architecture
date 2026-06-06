/**
 * Layer 4 — Adapter Layer（DPoP 証明検証）
 *
 * RFC 9449 に従い DPoP ヘッダー JWT を検証する。
 * リプレイ防止のため DpopReplayStore を併用する。
 *
 * クロックスキューと jti 再生窓は spec/scl.yaml properties.DpopJtiUniquenessWithinWindow に従う。
 */

import { jwtVerify, importJWK, calculateJwkThumbprint } from 'jose'
import type { JWK } from 'jose'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { DpopReplayStore } from '../../src/oauth2/ports/dpop-replay-store'

const CLOCK_SKEW_PAST_SECONDS = 60
const CLOCK_SKEW_FUTURE_SECONDS = 5
const JTI_REPLAY_WINDOW_SECONDS = 600 // 10 分

export interface DpopProofValidationResult {
  jkt: string
}

export async function verifyDpopProof(
  dpopHeader: string | undefined,
  expectedHtm: string,
  expectedHtu: string,
  options: {
    replayStore: DpopReplayStore
    /**
     * RFC 9449 §4.3 ath: base64url(SHA-256(access_token))。
     * Protected resource (例: /userinfo) で DPoP-bound AT を検証する時のみ指定する。
     */
    expectedAth?: string
  },
  now: Date = new Date(),
): Promise<DpopProofValidationResult | null> {
  if (!dpopHeader) return null

  let header: Record<string, unknown>
  try {
    const segs = dpopHeader.split('.')
    if (segs.length !== 3) throw new Error()
    header = JSON.parse(Buffer.from(segs[0], 'base64url').toString('utf8'))
  } catch {
    throw new OAuthError('invalid_dpop_proof', 'DPoP ヘッダーがパースできません')
  }

  if (header.typ !== 'dpop+jwt') {
    throw new OAuthError('invalid_dpop_proof', 'DPoP typ が dpop+jwt ではありません')
  }
  if (header.alg !== 'PS256' && header.alg !== 'ES256') {
    throw new OAuthError('invalid_dpop_proof', 'DPoP alg は PS256 / ES256 のみ許可')
  }
  const jwk = header.jwk as JWK | undefined
  if (!jwk || typeof jwk.kty !== 'string') {
    throw new OAuthError('invalid_dpop_proof', 'DPoP jwk ヘッダーが必要です')
  }

  const publicKey = await importJWK(jwk, header.alg as string)
  let payload: Record<string, unknown>
  try {
    const verified = await jwtVerify(dpopHeader, publicKey, {
      algorithms: ['PS256', 'ES256'],
    })
    payload = verified.payload as Record<string, unknown>
  } catch {
    throw new OAuthError('invalid_dpop_proof', 'DPoP 署名検証に失敗')
  }

  if (payload.htm !== expectedHtm) {
    throw new OAuthError('invalid_dpop_proof', `DPoP htm 不一致 (expected ${expectedHtm})`)
  }
  if (payload.htu !== expectedHtu) {
    throw new OAuthError('invalid_dpop_proof', `DPoP htu 不一致 (expected ${expectedHtu})`)
  }
  const iat = typeof payload.iat === 'number' ? payload.iat : 0
  const skew = now.getTime() / 1000 - iat
  if (skew > CLOCK_SKEW_PAST_SECONDS || skew < -CLOCK_SKEW_FUTURE_SECONDS) {
    throw new OAuthError('invalid_dpop_proof', 'DPoP iat がクロックスキュー範囲外')
  }
  const jti = payload.jti
  if (typeof jti !== 'string') {
    throw new OAuthError('invalid_dpop_proof', 'DPoP jti が必要です')
  }
  if (options.expectedAth !== undefined) {
    if (typeof payload.ath !== 'string') {
      throw new OAuthError('invalid_dpop_proof', 'DPoP ath が必要です')
    }
    if (payload.ath !== options.expectedAth) {
      throw new OAuthError('invalid_dpop_proof', 'DPoP ath がアクセストークンと一致しません')
    }
  }
  const isNew = await options.replayStore.recordIfNew(jti, JTI_REPLAY_WINDOW_SECONDS, now)
  if (!isNew) {
    throw new OAuthError('invalid_dpop_proof', 'DPoP jti のリプレイを検出')
  }

  const jkt = await calculateJwkThumbprint(jwk)
  return { jkt }
}
