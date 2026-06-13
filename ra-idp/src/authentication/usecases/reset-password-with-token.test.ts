/**
 * Layer 3 — Application Logic (reset-password-with-token テスト)
 */

import { describe, expect, it } from 'bun:test'
import { createHash } from 'crypto'

import { Argon2idPasswordHasher } from '../../../adapters/crypto/argon2id-password-hasher'
import { InMemoryPasswordHistoryRepository } from '../../../adapters/persistence/memory/password-history-repo'
import { InMemoryPasswordResetTokenStore } from '../../../adapters/persistence/memory/password-reset-token-store'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import type { DomainEvent, User } from '../../spec-bindings/schemas'
import { PasswordPolicyError } from './password-policy'
import {
  InvalidResetTokenError,
  PasswordReuseError,
  resetPasswordWithToken,
} from './reset-password-with-token'

async function setup(currentPassword: string) {
  const userRepo = new InMemoryUserRepository()
  const historyRepo = new InMemoryPasswordHistoryRepository()
  const tokenStore = new InMemoryPasswordResetTokenStore()
  const passwordHasher = new Argon2idPasswordHasher(8, 1)
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)
  const user: User = {
    sub: 'user-alice',
    preferred_username: 'alice',
    password_hash: await passwordHasher.hash(currentPassword),
    email: 'alice@example.com',
    email_verified: true,
    mfa_enrolled: false,
    created_at: '2024-01-01T00:00:00.000Z',
    updated_at: '2024-01-01T00:00:00.000Z',
  }
  await userRepo.save(user)
  return { userRepo, historyRepo, tokenStore, passwordHasher, events, emit, user }
}

async function saveToken(
  tokenStore: InMemoryPasswordResetTokenStore,
  sub: string,
  rawToken: string,
  ttlSeconds = 1800,
  now: Date = new Date(),
) {
  await tokenStore.save({
    sub,
    token_hash: createHash('sha256').update(rawToken, 'utf8').digest('hex'),
    created_at: now.toISOString(),
    expires_at: new Date(now.getTime() + ttlSeconds * 1000).toISOString(),
  })
}

describe('resetPasswordWithToken', () => {
  it('正常系: token を消費して password 更新 + PasswordChanged emit + history 追加', async () => {
    const h = await setup('current-password-1')
    const now = new Date('2026-06-13T12:00:00Z')
    await saveToken(h.tokenStore, h.user.sub, 'reset-tok-aaaa', 1800, now)
    const updated = await resetPasswordWithToken(
      { ...h, emit: (e) => h.events.push(e) },
      { token: 'reset-tok-aaaa', new_password: 'fresh-password-9182', now },
    )
    expect(await h.passwordHasher.verify('fresh-password-9182', updated.password_hash)).toBe(true)
    expect(h.events.map((e) => e.type)).toContain('PasswordChanged')
    const recent = await h.historyRepo.recent(h.user.sub, 5)
    expect(recent).toHaveLength(1)
    // 再消費は不可
    await expect(
      resetPasswordWithToken(
        { ...h, emit: (e) => h.events.push(e) },
        { token: 'reset-tok-aaaa', new_password: 'fresh-password-9182', now },
      ),
    ).rejects.toBeInstanceOf(InvalidResetTokenError)
  })

  it('期限切れ token は InvalidResetTokenError', async () => {
    const h = await setup('current-password-1')
    const issued = new Date('2026-06-13T11:00:00Z')
    const later = new Date('2026-06-13T12:00:00Z') // 1h 後 (TTL 30 分 を超過)
    await saveToken(h.tokenStore, h.user.sub, 'reset-tok-bbbb', 1800, issued)
    await expect(
      resetPasswordWithToken(
        { ...h, emit: (e) => h.events.push(e) },
        { token: 'reset-tok-bbbb', new_password: 'fresh-password-9182', now: later },
      ),
    ).rejects.toBeInstanceOf(InvalidResetTokenError)
  })

  it('不正な token は InvalidResetTokenError', async () => {
    const h = await setup('current-password-1')
    await expect(
      resetPasswordWithToken(
        { ...h, emit: (e) => h.events.push(e) },
        { token: 'wrong-token', new_password: 'fresh-password-9182' },
      ),
    ).rejects.toBeInstanceOf(InvalidResetTokenError)
  })

  it('policy 違反は PasswordPolicyError', async () => {
    const h = await setup('current-password-1')
    await saveToken(h.tokenStore, h.user.sub, 'reset-tok-cccc')
    await expect(
      resetPasswordWithToken(
        { ...h, emit: (e) => h.events.push(e) },
        { token: 'reset-tok-cccc', new_password: 'short' },
      ),
    ).rejects.toBeInstanceOf(PasswordPolicyError)
  })

  it('現在のパスワードを再利用すると PasswordReuseError', async () => {
    const h = await setup('current-password-1')
    await saveToken(h.tokenStore, h.user.sub, 'reset-tok-dddd')
    await expect(
      resetPasswordWithToken(
        { ...h, emit: (e) => h.events.push(e) },
        { token: 'reset-tok-dddd', new_password: 'current-password-1' },
      ),
    ).rejects.toBeInstanceOf(PasswordReuseError)
  })

  it('breachedPasswordChecker が hit を返したら breached 違反', async () => {
    const h = await setup('current-password-1')
    await saveToken(h.tokenStore, h.user.sub, 'reset-tok-eeee')
    await expect(
      resetPasswordWithToken(
        {
          ...h,
          emit: (e) => h.events.push(e),
          breachedPasswordChecker: {
            async isBreached() {
              return true
            },
          },
        },
        { token: 'reset-tok-eeee', new_password: 'fresh-password-9182' },
      ),
    ).rejects.toMatchObject({ name: 'PasswordPolicyError', violations: ['breached'] })
  })
})
