/**
 * Layer 3 — Application Logic (TOTP factor 登録)
 *
 * 二段階で動かす:
 *   1. start: ユーザに対して新しい base32 secret を採番し、otpauth:// URI と共に返す。
 *      この時点ではまだ MfaFactor を保存しない (登録未完了とする)。SPA は QR を表示
 *      し、ユーザに Authenticator 側で読み込ませる。secret は SPA セッションに一時
 *      的に保持する想定。
 *   2. confirm: SPA から secret + 1 回目の TOTP コードを受け取り、検証成功時にのみ
 *      MfaFactor を永続化し、User.mfa_enrolled を true にする。
 *
 * confirm に失敗した場合は何も保存しない（途中 secret を持ち続けない）。
 */

import { OAuthError } from '../../oauth2/protocol/oauth-error'
import type { MfaFactorRepository } from '../ports/mfa-factor-repository'
import type { UserRepository } from '../ports/user-repository'
import { buildOtpauthUri, generateTotpSecret, verifyTotp } from './totp'

export interface StartTotpEnrollmentInput {
  sub: string
  /** Authenticator 表示用のアカウント名 (preferred_username 等)。 */
  accountName: string
  /** Authenticator 表示用の発行者 (RA IdP の表示名)。 */
  issuer: string
}

export interface StartTotpEnrollmentResult {
  secretBase32: string
  otpauthUri: string
}

export function startTotpEnrollmentUseCase(
  input: StartTotpEnrollmentInput,
): StartTotpEnrollmentResult {
  const secretBase32 = generateTotpSecret()
  const otpauthUri = buildOtpauthUri({
    secretBase32,
    accountName: input.accountName,
    issuer: input.issuer,
  })
  return { secretBase32, otpauthUri }
}

export interface ConfirmTotpEnrollmentInput {
  sub: string
  /** start で発行された secret。SPA セッションから返ってくる。 */
  secretBase32: string
  /** ユーザが Authenticator アプリで生成した 6 桁コード。 */
  code: string
  /** ユーザが付ける表示名 (例 "iPhone Authenticator")。任意。 */
  label?: string
}

export async function confirmTotpEnrollmentUseCase(
  deps: { userRepo: UserRepository; mfaFactorRepo: MfaFactorRepository },
  input: ConfirmTotpEnrollmentInput,
  now: Date = new Date(),
): Promise<void> {
  const user = await deps.userRepo.findBySub(input.sub)
  if (!user) {
    throw new OAuthError('invalid_request', 'ユーザーが存在しません')
  }
  const ok = verifyTotp(input.secretBase32, input.code, Math.floor(now.getTime() / 1000))
  if (!ok) {
    throw new OAuthError('invalid_request', 'TOTP コードが正しくありません')
  }
  await deps.mfaFactorRepo.save({
    sub: input.sub,
    type: 'totp',
    secret: input.secretBase32,
    label: input.label,
    created_at: now.toISOString(),
  })
  if (!user.mfa_enrolled) {
    await deps.userRepo.save({ ...user, mfa_enrolled: true, updated_at: now.toISOString() })
  }
}
