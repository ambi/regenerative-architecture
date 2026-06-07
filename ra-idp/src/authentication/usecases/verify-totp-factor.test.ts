import { describe, expect, it } from 'bun:test'
import { InMemoryMfaFactorRepository } from '../../../adapters/persistence/memory/mfa-factor-repo'
import { generateTotp } from './totp'
import { verifyTotpFactorUseCase } from './verify-totp-factor'

const SECRET = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'

describe('verifyTotpFactorUseCase', () => {
  it('factor 未登録なら no_factor', async () => {
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    const result = await verifyTotpFactorUseCase(
      { mfaFactorRepo },
      { sub: 'user-1', code: '000000' },
    )
    expect(result).toEqual({ ok: false, reason: 'no_factor' })
  })

  it('正しいコードを受理し last_used_at を更新する', async () => {
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    await mfaFactorRepo.save({
      sub: 'user-1',
      type: 'totp',
      secret: SECRET,
      created_at: new Date('2024-01-01T00:00:00Z').toISOString(),
    })
    const now = new Date('2025-06-01T00:00:00Z')
    const code = generateTotp(SECRET, Math.floor(now.getTime() / 1000))

    const result = await verifyTotpFactorUseCase({ mfaFactorRepo }, { sub: 'user-1', code }, now)
    expect(result).toEqual({ ok: true })

    const stored = await mfaFactorRepo.find('user-1', 'totp')
    expect(stored?.last_used_at).toBe(now.toISOString())
  })

  it('誤ったコードは invalid_code', async () => {
    const mfaFactorRepo = new InMemoryMfaFactorRepository()
    await mfaFactorRepo.save({
      sub: 'user-1',
      type: 'totp',
      secret: SECRET,
      created_at: new Date('2024-01-01T00:00:00Z').toISOString(),
    })
    const result = await verifyTotpFactorUseCase(
      { mfaFactorRepo },
      { sub: 'user-1', code: '000000' },
      new Date('2025-06-01T00:00:00Z'),
    )
    expect(result).toEqual({ ok: false, reason: 'invalid_code' })
  })
})
