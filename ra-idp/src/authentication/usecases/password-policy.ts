/**
 * Layer 3 — Application Logic（パスワードポリシー）
 *
 * 仕様核は spec/scl.yaml `annotations.password_policy`。ここでは TypeScript
 * 実装からポリシーを参照可能にするための双子定義を置く。SCL と本ファイルの値が
 * 乖離すると spec↔impl drift になるため、check:coherence で突き合わせる。
 *
 * NIST SP 800-63B-4 §3.1.1.2 に沿って composition rule（文字種混在）と periodic
 * rotation は採用しない。詳細は ADR-026 を参照。
 */

import { COMMON_PASSWORDS } from './common-passwords'

export type PasswordPolicyViolation =
  | 'too_short'
  | 'too_long'
  | 'similar_to_identifier'
  | 'common_password'

export const PASSWORD_POLICY = {
  minLength: 12,
  maxLength: 128,
  forbidUserIdentifierSimilarity: true,
  commonPasswordDictionary: 'bundled',
} as const

/**
 * 類似判定で「短すぎて誤検知になる」識別子を弾く下限。
 * 識別子が 4 文字未満なら類似チェックを行わない（例: 名前のイニシャル "ab" 等）。
 */
const MIN_IDENTIFIER_SUBSTRING_LENGTH = 4

export interface PasswordContext {
  /** preferred_username 等のユーザー識別子。 */
  username?: string
  /** メールアドレス全体。local-part も別途比較する。 */
  email?: string
}

export type PasswordPolicyResult =
  | { ok: true }
  | { ok: false; violations: PasswordPolicyViolation[] }

export function validatePassword(plain: string, context?: PasswordContext): PasswordPolicyResult {
  const violations: PasswordPolicyViolation[] = []
  if (plain.length < PASSWORD_POLICY.minLength) violations.push('too_short')
  if (plain.length > PASSWORD_POLICY.maxLength) violations.push('too_long')
  if (isSimilarToIdentifier(plain, context)) violations.push('similar_to_identifier')
  if (isCommonPassword(plain)) violations.push('common_password')
  return violations.length === 0 ? { ok: true } : { ok: false, violations }
}

export class PasswordPolicyError extends Error {
  constructor(public readonly violations: PasswordPolicyViolation[]) {
    super(`password policy violated: ${violations.join(', ')}`)
    this.name = 'PasswordPolicyError'
  }
}

function isSimilarToIdentifier(plain: string, context: PasswordContext | undefined): boolean {
  if (!context) return false
  const lowered = plain.toLowerCase()
  const candidates: string[] = []
  if (context.username) candidates.push(context.username)
  if (context.email) {
    candidates.push(context.email)
    const at = context.email.indexOf('@')
    if (at > 0) candidates.push(context.email.slice(0, at))
  }
  for (const raw of candidates) {
    const id = raw.toLowerCase()
    if (id.length < MIN_IDENTIFIER_SUBSTRING_LENGTH) continue
    if (lowered.includes(id)) return true
    if (id.includes(lowered) && lowered.length >= MIN_IDENTIFIER_SUBSTRING_LENGTH) return true
  }
  return false
}

function isCommonPassword(plain: string): boolean {
  return COMMON_PASSWORDS.has(plain.toLowerCase())
}
