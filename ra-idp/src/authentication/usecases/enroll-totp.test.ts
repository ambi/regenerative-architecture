import { describe, expect, it } from 'bun:test'
import { InMemoryMfaFactorRepository } from '../../../adapters/persistence/memory/mfa-factor-repo'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import type { User } from '../../spec-bindings/schemas'
import { confirmTotpEnrollmentUseCase, startTotpEnrollmentUseCase } from './enroll-totp'
import { generateTotp } from './totp'

function makeUser(): User {
  return {
    sub: 'user-1',
    tenant_id: 'default',
    preferred_username: 'alice',
    password_hash: 'x',
    email_verified: false,
    mfa_enrolled: false,
    roles: [],
    created_at: new Date('2024-01-01T00:00:00Z').toISOString(),
    updated_at: new Date('2024-01-01T00:00:00Z').toISOString(),
  }
}

describe('startTotpEnrollmentUseCase', () => {
  it('secret と otpauth:// URI を返す', () => {
    const result = startTotpEnrollmentUseCase({
      sub: 'user-1',
      accountName: 'alice@example.com',
      issuer: 'RA IdP',
    })
    expect(result.secretBase32).toMatch(/^[A-Z2-7]{32}$/)
    expect(result.otpauthUri).toContain('otpauth://totp/')
    expect(result.otpauthUri).toContain(`secret=${result.secretBase32}`)
  })
})

describe('confirmTotpEnrollmentUseCase', () => {
  it('正しいコードで保存し User.mfa_enrolled を true にする', async () => {
    const userRepo = new InMemoryUserRepository()
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    await userRepo.save(makeUser())
    const { secretBase32 } = startTotpEnrollmentUseCase({
      sub: 'user-1',
      accountName: 'alice',
      issuer: 'RA IdP',
    })
    const now = new Date('2025-06-01T00:00:00Z')
    const code = generateTotp(secretBase32, Math.floor(now.getTime() / 1000))

    await confirmTotpEnrollmentUseCase(
      { userRepo, mfaFactorRepo },
      { sub: 'user-1', secretBase32, code, label: 'iPhone' },
      now,
    )

    const stored = await mfaFactorRepo.find('user-1', 'totp')
    expect(stored).not.toBeNull()
    expect(stored?.secret).toBe(secretBase32)
    expect(stored?.label).toBe('iPhone')

    const updated = await userRepo.findBySub('user-1')
    expect(updated?.mfa_enrolled).toBe(true)
  })

  it('誤ったコードは invalid_request で拒否し factor を保存しない', async () => {
    const userRepo = new InMemoryUserRepository()
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    await userRepo.save(makeUser())
    const { secretBase32 } = startTotpEnrollmentUseCase({
      sub: 'user-1',
      accountName: 'alice',
      issuer: 'RA IdP',
    })

    await expect(
      confirmTotpEnrollmentUseCase(
        { userRepo, mfaFactorRepo },
        { sub: 'user-1', secretBase32, code: '000000' },
        new Date('2025-06-01T00:00:00Z'),
      ),
    ).rejects.toMatchObject({ code: 'invalid_request' })

    expect(await mfaFactorRepo.find('user-1', 'totp')).toBeNull()
    expect((await userRepo.findBySub('user-1'))?.mfa_enrolled).toBe(false)
  })

  it('存在しないユーザは invalid_request', async () => {
    const userRepo = new InMemoryUserRepository()
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    await expect(
      confirmTotpEnrollmentUseCase(
        { userRepo, mfaFactorRepo },
        { sub: 'ghost', secretBase32: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ', code: '000000' },
      ),
    ).rejects.toMatchObject({ code: 'invalid_request' })
  })
})
