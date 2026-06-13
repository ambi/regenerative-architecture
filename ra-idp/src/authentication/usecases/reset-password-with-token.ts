/**
 * Layer 3 — Application Logic (reset password with token use case)
 *
 * ADR-030: 単発消費トークンを受け取り、change-password と同じ policy
 * パイプライン (validatePasswordAsync + PasswordHistoryRepository +
 * BreachedPasswordChecker) を通って新パスワードを適用する。
 *
 * フロー:
 *   1. token を SHA-256 で hash 化 → store.consume() で原子的に取り出す
 *   2. record が無い / 期限切れなら InvalidResetTokenError
 *   3. user 解決 (sub) → 無ければ InvalidResetTokenError 相当 (race)
 *   4. validatePasswordAsync で policy + breached check
 *   5. history depth 件と照合 (PasswordReuseError)
 *   6. hash → user 更新 → history 追加 → PasswordChanged emit
 */

import { createHash } from 'crypto'
import type { DomainEvent, User } from '../../spec-bindings/schemas'
import type { BreachedPasswordChecker } from '../ports/breached-password-checker'
import type { PasswordHasher } from '../ports/password-hasher'
import type { PasswordHistoryRepository } from '../ports/password-history-repository'
import type { PasswordResetTokenStore } from '../ports/password-reset-token-store'
import type { UserRepository } from '../ports/user-repository'
import { PASSWORD_POLICY, PasswordPolicyError, validatePasswordAsync } from './password-policy'

export class InvalidResetTokenError extends Error {
  constructor() {
    super('reset token is invalid or expired')
    this.name = 'InvalidResetTokenError'
  }
}

export class PasswordReuseError extends Error {
  constructor() {
    super('new password matches a recent password')
    this.name = 'PasswordReuseError'
  }
}

export interface ResetPasswordWithTokenDeps {
  userRepo: UserRepository
  tokenStore: PasswordResetTokenStore
  passwordHasher: PasswordHasher
  historyRepo: PasswordHistoryRepository
  breachedPasswordChecker?: BreachedPasswordChecker
  emit: (e: DomainEvent) => void
  /** 将来テナント別ポリシーで上書きする差し込み点。 */
  historyDepth?: number
}

export interface ResetPasswordWithTokenInput {
  token: string
  new_password: string
  now?: Date
}

export async function resetPasswordWithToken(
  deps: ResetPasswordWithTokenDeps,
  input: ResetPasswordWithTokenInput,
): Promise<User> {
  const now = input.now ?? new Date()
  const tokenHash = createHash('sha256').update(input.token, 'utf8').digest('hex')
  const record = await deps.tokenStore.consume(tokenHash, now)
  if (!record) throw new InvalidResetTokenError()

  const user = await deps.userRepo.findBySub(record.sub)
  if (!user) throw new InvalidResetTokenError()

  const result = await validatePasswordAsync(
    input.new_password,
    { username: user.preferred_username, email: user.email },
    deps.breachedPasswordChecker,
  )
  if (!result.ok) throw new PasswordPolicyError(result.violations)

  const depth = deps.historyDepth ?? PASSWORD_POLICY.historyDepth
  const recent = await deps.historyRepo.recent(user.sub, depth)
  for (const entry of recent) {
    if (await deps.passwordHasher.verify(input.new_password, entry.encoded)) {
      throw new PasswordReuseError()
    }
  }
  if (await deps.passwordHasher.verify(input.new_password, user.password_hash)) {
    throw new PasswordReuseError()
  }

  const newHash = await deps.passwordHasher.hash(input.new_password)
  const updated: User = { ...user, password_hash: newHash, updated_at: now.toISOString() }
  await deps.userRepo.save(updated)
  await deps.historyRepo.add(user.sub, newHash, now)
  deps.emit({ type: 'PasswordChanged', occurredAt: now.toISOString(), sub: user.sub })
  return updated
}
