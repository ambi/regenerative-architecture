/**
 * Layer 3 — Application Logic（パスワードポリシー）
 *
 * 仕様核は spec/scl.yaml `annotations.password_policy`。ここでは TypeScript
 * 実装からポリシーを参照可能にするための双子定義を置く。SCL と本ファイルの値が
 * 乖離すると spec↔impl drift になるため、check:coherence で突き合わせる。
 */

export type PasswordPolicyViolation = 'too_short' | 'too_long'

export const PASSWORD_POLICY = {
  minLength: 12,
  maxLength: 128,
} as const

export type PasswordPolicyResult =
  | { ok: true }
  | { ok: false; violations: PasswordPolicyViolation[] }

export function validatePassword(plain: string): PasswordPolicyResult {
  const violations: PasswordPolicyViolation[] = []
  if (plain.length < PASSWORD_POLICY.minLength) violations.push('too_short')
  if (plain.length > PASSWORD_POLICY.maxLength) violations.push('too_long')
  return violations.length === 0 ? { ok: true } : { ok: false, violations }
}

export class PasswordPolicyError extends Error {
  constructor(public readonly violations: PasswordPolicyViolation[]) {
    super(`password policy violated: ${violations.join(', ')}`)
    this.name = 'PasswordPolicyError'
  }
}
