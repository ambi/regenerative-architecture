/**
 * Layer 4 — Adapter Layer (in-memory LoginAttemptThrottle 契約テスト)
 */

import { describe, expect, it } from 'bun:test'

import { InMemoryLoginAttemptThrottle, type LoginThrottleConfigs } from './login-attempt-throttle'

const CONFIGS: LoginThrottleConfigs = {
  account: { maxFailures: 3, windowSeconds: 60, lockoutSeconds: 120 },
  ip: { maxFailures: 5, windowSeconds: 60, lockoutSeconds: 60 },
}

describe('InMemoryLoginAttemptThrottle', () => {
  it('しきい値未満なら tryAcquire は allowed', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const now = new Date('2026-01-01T00:00:00Z')
    expect(await t.tryAcquire('account', 'alice', now)).toEqual({ allowed: true })
    await t.recordFailure('account', 'alice', now)
    expect(await t.tryAcquire('account', 'alice', now)).toEqual({ allowed: true })
  })

  it('しきい値到達でロックされ Retry-After を返す', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const now = new Date('2026-01-01T00:00:00Z')
    expect((await t.recordFailure('account', 'alice', now)).locked).toBe(false)
    expect((await t.recordFailure('account', 'alice', now)).locked).toBe(false)
    const third = await t.recordFailure('account', 'alice', now)
    expect(third.locked).toBe(true)
    expect(third.retryAfterSeconds).toBe(120)
    const acquire = await t.tryAcquire('account', 'alice', now)
    expect(acquire.allowed).toBe(false)
    expect(acquire.retryAfterSeconds).toBe(120)
  })

  it('ロック中の他 key には影響しない', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const now = new Date('2026-01-01T00:00:00Z')
    await t.recordFailure('account', 'alice', now)
    await t.recordFailure('account', 'alice', now)
    await t.recordFailure('account', 'alice', now) // alice ロック
    expect(await t.tryAcquire('account', 'bob', now)).toEqual({ allowed: true })
  })

  it('lockoutSeconds 経過後は自動解除', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const t0 = new Date('2026-01-01T00:00:00Z')
    for (let i = 0; i < 3; i++) await t.recordFailure('account', 'alice', t0)
    const later = new Date(t0.getTime() + 121 * 1000)
    expect(await t.tryAcquire('account', 'alice', later)).toEqual({ allowed: true })
  })

  it('window を跨いだ古い失敗は持ち越さない', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const t0 = new Date('2026-01-01T00:00:00Z')
    await t.recordFailure('account', 'alice', t0)
    await t.recordFailure('account', 'alice', t0)
    const later = new Date(t0.getTime() + 61 * 1000) // window 外
    const result = await t.recordFailure('account', 'alice', later)
    expect(result.locked).toBe(false)
  })

  it('recordSuccess は account の counter と lock をクリアする', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const now = new Date('2026-01-01T00:00:00Z')
    for (let i = 0; i < 3; i++) await t.recordFailure('account', 'alice', now)
    await t.recordSuccess('account', 'alice')
    expect(await t.tryAcquire('account', 'alice', now)).toEqual({ allowed: true })
  })

  it('account と ip は独立に集計される', async () => {
    const t = new InMemoryLoginAttemptThrottle(CONFIGS)
    const now = new Date('2026-01-01T00:00:00Z')
    for (let i = 0; i < 3; i++) await t.recordFailure('account', 'alice', now) // account ロック
    expect((await t.tryAcquire('account', 'alice', now)).allowed).toBe(false)
    expect((await t.tryAcquire('ip', '1.2.3.4', now)).allowed).toBe(true)
  })
})
