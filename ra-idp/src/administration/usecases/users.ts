/**
 * Layer 3 — Application Logic（管理 API: User lifecycle）
 *
 * 仕様核は spec/scl.yaml `interfaces.{Create,Update,Disable,Enable}AdminUser` /
 * `events.User{Created,Updated,Disabled,Enabled}` / 認可ルール `AdminUser*`。
 * 認可境界 (admin role 検査・CSRF) は HTTP adapter 側で行う。本 use case は
 * UserRepository / PasswordHasher / PasswordHistory に閉じた純粋ロジックを置く。
 * 詳細は ADR-031。
 */

import { randomUUID } from 'crypto'

import type { PasswordHasher } from '../../authentication/ports/password-hasher'
import type { PasswordHistoryRepository } from '../../authentication/ports/password-history-repository'
import type { UserRepository } from '../../authentication/ports/user-repository'
import {
  PasswordPolicyError,
  validatePassword,
} from '../../authentication/usecases/password-policy'
import { type DomainEvent, type User, UserSchema } from '../../spec-bindings/schemas'

export class UserNotFoundError extends Error {
  constructor(public readonly sub: string) {
    super(`user not found: ${sub}`)
    this.name = 'UserNotFoundError'
  }
}

export class UsernameConflictError extends Error {
  constructor(public readonly username: string) {
    super(`preferred username already exists: ${username}`)
    this.name = 'UsernameConflictError'
  }
}

export class InvalidRoleError extends Error {
  constructor() {
    super('role must not be empty')
    this.name = 'InvalidRoleError'
  }
}

export interface AdminUserDeps {
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  passwordHistoryRepo: PasswordHistoryRepository
  emit: (event: DomainEvent) => void
}

export interface CreateAdminUserInput {
  actorSub: string
  preferred_username: string
  password: string
  name?: string
  email?: string
  email_verified?: boolean
  roles?: string[]
  now?: Date
}

export async function createAdminUser(
  deps: AdminUserDeps,
  input: CreateAdminUserInput,
): Promise<User> {
  const username = input.preferred_username.trim()
  if (!username) throw new Error('preferred username is required')

  const existing = await deps.userRepo.findByUsername('default', username)
  if (existing) throw new UsernameConflictError(username)

  const policy = validatePassword(input.password, {
    username,
    email: input.email,
  })
  if (!policy.ok) throw new PasswordPolicyError(policy.violations)

  const roles = normalizeRoles(input.roles ?? [])
  const now = input.now ?? new Date()
  const passwordHash = await deps.passwordHasher.hash(input.password)
  const user = UserSchema.parse({
    sub: `user_${randomUUID()}`,
    tenant_id: 'default',
    preferred_username: username,
    password_hash: passwordHash,
    name: input.name,
    email: input.email,
    email_verified: input.email_verified ?? false,
    mfa_enrolled: false,
    roles,
    created_at: now.toISOString(),
    updated_at: now.toISOString(),
  })
  await deps.userRepo.save(user)
  await deps.passwordHistoryRepo.add(user.sub, passwordHash, now)
  deps.emit({
    type: 'UserCreated',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    targetSub: user.sub,
  })
  return user
}

export interface UpdateAdminUserInput {
  actorSub: string
  sub: string
  preferred_username?: string
  name?: string
  email?: string
  email_verified?: boolean
  roles?: string[]
  now?: Date
}

export async function updateAdminUser(
  deps: AdminUserDeps,
  input: UpdateAdminUserInput,
): Promise<User> {
  const user = await deps.userRepo.findBySub(input.sub)
  if (!user) throw new UserNotFoundError(input.sub)

  const next: User = { ...user }
  const changed: string[] = []

  if (input.preferred_username !== undefined) {
    const username = input.preferred_username.trim()
    if (!username) throw new Error('preferred username must not be empty')
    if (username !== user.preferred_username) {
      const collision = await deps.userRepo.findByUsername('default', username)
      if (collision && collision.sub !== user.sub) {
        throw new UsernameConflictError(username)
      }
      next.preferred_username = username
      changed.push('preferred_username')
    }
  }
  if (input.name !== undefined && input.name !== user.name) {
    next.name = input.name
    changed.push('name')
  }
  if (input.email !== undefined && input.email !== user.email) {
    next.email = input.email
    changed.push('email')
  }
  if (input.email_verified !== undefined && input.email_verified !== user.email_verified) {
    next.email_verified = input.email_verified
    changed.push('email_verified')
  }
  if (input.roles !== undefined) {
    const roles = normalizeRoles(input.roles)
    if (!sameRoles(roles, user.roles)) {
      next.roles = roles
      changed.push('roles')
    }
  }
  if (changed.length === 0) return user

  const now = input.now ?? new Date()
  next.updated_at = now.toISOString()
  const validated = UserSchema.parse(next)
  await deps.userRepo.save(validated)
  deps.emit({
    type: 'UserUpdated',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    targetSub: user.sub,
    changedFields: changed,
  })
  return validated
}

export interface SetUserDisabledInput {
  actorSub: string
  sub: string
  disabled: boolean
  now?: Date
}

export async function setAdminUserDisabled(
  deps: AdminUserDeps,
  input: SetUserDisabledInput,
): Promise<User> {
  const user = await deps.userRepo.findBySub(input.sub)
  if (!user) throw new UserNotFoundError(input.sub)

  if (input.disabled && user.disabled_at) return user
  if (!input.disabled && !user.disabled_at) return user

  const now = input.now ?? new Date()
  const next: User = {
    ...user,
    disabled_at: input.disabled ? now.toISOString() : undefined,
    updated_at: now.toISOString(),
  }
  const validated = UserSchema.parse(next)
  await deps.userRepo.save(validated)
  deps.emit({
    type: input.disabled ? 'UserDisabled' : 'UserEnabled',
    occurredAt: now.toISOString(),
    actorSub: input.actorSub,
    targetSub: user.sub,
  })
  return validated
}

function normalizeRoles(roles: string[]): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of roles) {
    const role = raw.trim()
    if (!role) throw new InvalidRoleError()
    if (!seen.has(role)) {
      seen.add(role)
      out.push(role)
    }
  }
  return out.sort()
}

function sameRoles(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false
  for (let i = 0; i < left.length; i++) {
    if (left[i] !== right[i]) return false
  }
  return true
}
