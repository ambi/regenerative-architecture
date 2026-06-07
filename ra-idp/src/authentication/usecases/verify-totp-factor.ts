/**
 * Layer 3 — Application Logic (TOTP factor 検証)
 *
 * ログイン中のユーザに対し、保存済みの TOTP factor を使ってコードを検証する。
 * 検証成功時には last_used_at を更新する。検証ロジック自体は totp.ts (pure) に
 * 委ねており、本ユースケースは MfaFactorRepository とのつなぎ込みのみ担う。
 */

import type { MfaFactorRepository } from '../ports/mfa-factor-repository'
import { verifyTotp } from './totp'

export interface VerifyTotpFactorInput {
  sub: string
  code: string
}

export type VerifyTotpFactorResult =
  | { ok: true }
  | { ok: false; reason: 'no_factor' | 'invalid_code' }

export async function verifyTotpFactorUseCase(
  deps: { mfaFactorRepo: MfaFactorRepository },
  input: VerifyTotpFactorInput,
  now: Date = new Date(),
): Promise<VerifyTotpFactorResult> {
  const factor = await deps.mfaFactorRepo.find(input.sub, 'totp')
  if (!factor || !factor.secret) {
    return { ok: false, reason: 'no_factor' }
  }
  const ok = verifyTotp(factor.secret, input.code, Math.floor(now.getTime() / 1000))
  if (!ok) {
    return { ok: false, reason: 'invalid_code' }
  }
  await deps.mfaFactorRepo.save({ ...factor, last_used_at: now.toISOString() })
  return { ok: true }
}
