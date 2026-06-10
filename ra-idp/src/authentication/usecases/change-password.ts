/**
 * Layer 3 — Application Logic（change-password use case）
 *
 * 認証済みユーザーが自身のパスワードを変更する。SCL `interfaces.ChangePassword` /
 * `events.PasswordChanged` / invariant `PasswordHistoryNoReuse` の双子。
 *
 * フロー:
 *   1. sub から User を解決
 *   2. current_password を hash と verify
 *   3. new_password を password_policy で検証
 *   4. 直近 history_depth 件と逐次 verify（一致したら拒否）
 *   5. new_password を hash → save → history.add → PasswordChanged emit
 */

import type { DomainEvent, User } from '../../spec-bindings/schemas'
import type { PasswordHasher } from '../ports/password-hasher'
import type { PasswordHistoryRepository } from '../ports/password-history-repository'
import type { UserRepository } from '../ports/user-repository'
import { PASSWORD_POLICY, PasswordPolicyError, validatePassword } from './password-policy'

export class CurrentPasswordMismatchError extends Error {
  constructor() {
    super('current password does not match')
    this.name = 'CurrentPasswordMismatchError'
  }
}

export class PasswordReuseError extends Error {
  constructor() {
    super('new password matches a recent password')
    this.name = 'PasswordReuseError'
  }
}

export class UserNotFoundError extends Error {
  constructor(public readonly sub: string) {
    super(`user not found: ${sub}`)
    this.name = 'UserNotFoundError'
  }
}

export interface ChangePasswordDeps {
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  historyRepo: PasswordHistoryRepository
  emit: (e: DomainEvent) => void
  /** 将来テナント別ポリシーで上書きする際の差し込み点。省略時は global default。 */
  historyDepth?: number
}

export interface ChangePasswordInput {
  sub: string
  current_password: string
  new_password: string
  now?: Date
}

export async function changePassword(
  deps: ChangePasswordDeps,
  input: ChangePasswordInput,
): Promise<User> {
  const user = await deps.userRepo.findBySub(input.sub)
  if (!user) throw new UserNotFoundError(input.sub)

  const currentOk = await deps.passwordHasher.verify(input.current_password, user.password_hash)
  if (!currentOk) throw new CurrentPasswordMismatchError()

  const result = validatePassword(input.new_password, {
    username: user.preferred_username,
    email: user.email,
  })
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

  const now = input.now ?? new Date()
  const newHash = await deps.passwordHasher.hash(input.new_password)
  const updated: User = { ...user, password_hash: newHash, updated_at: now.toISOString() }
  await deps.userRepo.save(updated)
  await deps.historyRepo.add(user.sub, newHash, now)
  deps.emit({ type: 'PasswordChanged', occurredAt: now.toISOString(), sub: user.sub })
  return updated
}
