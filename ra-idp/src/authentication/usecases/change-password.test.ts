import { describe, expect, it } from 'bun:test'
import { Argon2idPasswordHasher } from '../../../adapters/crypto/argon2id-password-hasher'
import { InMemoryPasswordHistoryRepository } from '../../../adapters/persistence/memory/password-history-repo'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import type { BreachedPasswordChecker } from '../ports/breached-password-checker'
import type { DomainEvent, User } from '../../spec-bindings/schemas'
import {
  CurrentPasswordMismatchError,
  PasswordReuseError,
  UserNotFoundError,
  changePassword,
} from './change-password'
import { PASSWORD_POLICY, PasswordPolicyError } from './password-policy'

interface Harness {
  userRepo: InMemoryUserRepository
  historyRepo: InMemoryPasswordHistoryRepository
  passwordHasher: Argon2idPasswordHasher
  events: DomainEvent[]
  sub: string
}

async function setupUser(initialPassword: string): Promise<Harness> {
  const userRepo = new InMemoryUserRepository()
  const historyRepo = new InMemoryPasswordHistoryRepository()
  // Argon2id は slow なので低コストプロファイルでテストする (memoryCost KiB, timeCost)
  const passwordHasher = new Argon2idPasswordHasher(8, 1)
  const hash = await passwordHasher.hash(initialPassword)
  const user: User = {
    sub: 'user-1',
    preferred_username: 'alice',
    password_hash: hash,
    email_verified: false,
    mfa_enrolled: false,
    created_at: '2024-01-01T00:00:00.000Z',
    updated_at: '2024-01-01T00:00:00.000Z',
  }
  await userRepo.save(user)
  return { userRepo, historyRepo, passwordHasher, events: [], sub: user.sub }
}

describe('changePassword', () => {
  it('現パス一致 + ポリシー合格 + 履歴未一致なら hash を更新し PasswordChanged を emit', async () => {
    const h = await setupUser('demo-password-1234')
    const now = new Date('2025-06-10T00:00:00Z')

    const updated = await changePassword(
      { ...h, emit: (e) => h.events.push(e) },
      { sub: h.sub, current_password: 'demo-password-1234', new_password: 'fresh-pass-9182', now },
    )

    expect(await h.passwordHasher.verify('fresh-pass-9182', updated.password_hash)).toBe(true)
    const persisted = await h.userRepo.findBySub(h.sub)
    expect(persisted?.password_hash).toBe(updated.password_hash)
    expect(persisted?.updated_at).toBe(now.toISOString())
    expect(h.events).toEqual([
      { type: 'PasswordChanged', occurredAt: now.toISOString(), sub: h.sub },
    ])
    const recent = await h.historyRepo.recent(h.sub, PASSWORD_POLICY.historyDepth)
    expect(recent).toHaveLength(1)
    expect(await h.passwordHasher.verify('fresh-pass-9182', recent[0].encoded)).toBe(true)
  })

  it('現パスが一致しないと CurrentPasswordMismatchError', async () => {
    const h = await setupUser('demo-password-1234')
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        { sub: h.sub, current_password: 'wrong', new_password: 'fresh-pass-9182' },
      ),
    ).rejects.toBeInstanceOf(CurrentPasswordMismatchError)
    expect(h.events).toEqual([])
  })

  it('新パスがポリシー違反なら PasswordPolicyError', async () => {
    const h = await setupUser('demo-password-1234')
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        { sub: h.sub, current_password: 'demo-password-1234', new_password: 'short' },
      ),
    ).rejects.toBeInstanceOf(PasswordPolicyError)
    expect(h.events).toEqual([])
  })

  it('現在のパスワードと同じ新パスは再利用として PasswordReuseError', async () => {
    const h = await setupUser('demo-password-1234')
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        {
          sub: h.sub,
          current_password: 'demo-password-1234',
          new_password: 'demo-password-1234',
        },
      ),
    ).rejects.toBeInstanceOf(PasswordReuseError)
    expect(h.events).toEqual([])
  })

  it('直近 historyDepth 件のいずれかと一致したら PasswordReuseError', async () => {
    const h = await setupUser('demo-password-1234')
    const passwords = [
      'pw-history-aaaa-1',
      'pw-history-bbbb-2',
      'pw-history-cccc-3',
      'pw-history-dddd-4',
      'pw-history-eeee-5',
    ]
    let current = 'demo-password-1234'
    for (const next of passwords) {
      await changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        { sub: h.sub, current_password: current, new_password: next },
      )
      current = next
    }
    // historyDepth=5 件を回したのでこの時点では最初の demo-password-1234 は履歴外。
    // しかし直近 5 件の "pw-history-aaaa-1" は history に残るため、再選択は失敗する。
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        { sub: h.sub, current_password: current, new_password: 'pw-history-aaaa-1' },
      ),
    ).rejects.toBeInstanceOf(PasswordReuseError)
  })

  it('history depth を超えた古いパスワードは再利用可能', async () => {
    const h = await setupUser('demo-password-1234')
    // depth=2 で usecase を実行し、3 世代回した後に最古を再利用可能なことを確認
    const opts = {
      ...h,
      emit: (e: DomainEvent) => h.events.push(e),
      historyDepth: 2,
    }
    await changePassword(opts, {
      sub: h.sub,
      current_password: 'demo-password-1234',
      new_password: 'rotate-aaaa-1234',
    })
    await changePassword(opts, {
      sub: h.sub,
      current_password: 'rotate-aaaa-1234',
      new_password: 'rotate-bbbb-1234',
    })
    await changePassword(opts, {
      sub: h.sub,
      current_password: 'rotate-bbbb-1234',
      new_password: 'rotate-cccc-1234',
    })
    // depth=2 で見るのは直近 2 件 (cccc, bbbb)。aaaa は履歴外で再利用可能。
    const reused = await changePassword(opts, {
      sub: h.sub,
      current_password: 'rotate-cccc-1234',
      new_password: 'rotate-aaaa-1234',
    })
    expect(await h.passwordHasher.verify('rotate-aaaa-1234', reused.password_hash)).toBe(true)
  })

  it('存在しない sub は UserNotFoundError', async () => {
    const h = await setupUser('demo-password-1234')
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e) },
        { sub: 'ghost', current_password: 'demo-password-1234', new_password: 'fresh-pass-9182' },
      ),
    ).rejects.toBeInstanceOf(UserNotFoundError)
  })

  it('breachedPasswordChecker がヒットを返したら PasswordPolicyError([breached])', async () => {
    const h = await setupUser('demo-password-1234')
    const seen: string[] = []
    const breachedPasswordChecker: BreachedPasswordChecker = {
      async isBreached(plain) {
        seen.push(plain)
        return plain === 'leaked-password-9999'
      },
    }
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e), breachedPasswordChecker },
        {
          sub: h.sub,
          current_password: 'demo-password-1234',
          new_password: 'leaked-password-9999',
        },
      ),
    ).rejects.toMatchObject({
      name: 'PasswordPolicyError',
      violations: ['breached'],
    })
    expect(seen).toEqual(['leaked-password-9999'])
    expect(h.events).toEqual([])
  })

  it('breachedPasswordChecker が false を返せば従来通り成功する', async () => {
    const h = await setupUser('demo-password-1234')
    const breachedPasswordChecker: BreachedPasswordChecker = {
      async isBreached() {
        return false
      },
    }
    const updated = await changePassword(
      { ...h, emit: (e) => h.events.push(e), breachedPasswordChecker },
      {
        sub: h.sub,
        current_password: 'demo-password-1234',
        new_password: 'fresh-pass-9182',
      },
    )
    expect(await h.passwordHasher.verify('fresh-pass-9182', updated.password_hash)).toBe(true)
  })

  it('bundled policy 違反があるときは breachedPasswordChecker を呼ばない (早期失敗)', async () => {
    const h = await setupUser('demo-password-1234')
    let called = false
    const breachedPasswordChecker: BreachedPasswordChecker = {
      async isBreached() {
        called = true
        return true
      },
    }
    await expect(
      changePassword(
        { ...h, emit: (e) => h.events.push(e), breachedPasswordChecker },
        { sub: h.sub, current_password: 'demo-password-1234', new_password: 'short' },
      ),
    ).rejects.toBeInstanceOf(PasswordPolicyError)
    expect(called).toBe(false)
  })
})
