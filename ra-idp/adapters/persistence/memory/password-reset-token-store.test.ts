/**
 * Layer 4 — Adapter Layer (in-memory PasswordResetTokenStore 契約)
 */

import { describe, expect, it } from 'bun:test'

import { InMemoryPasswordResetTokenStore } from './password-reset-token-store'

function record(sub: string, hash: string, ttlSeconds = 1800, now = new Date()) {
  return {
    sub,
    token_hash: hash,
    created_at: now.toISOString(),
    expires_at: new Date(now.getTime() + ttlSeconds * 1000).toISOString(),
  }
}

describe('InMemoryPasswordResetTokenStore', () => {
  it('save → consume で record を返し、二度目は null', async () => {
    const s = new InMemoryPasswordResetTokenStore()
    await s.save(record('alice', 'h1'))
    const first = await s.consume('h1', new Date())
    expect(first?.sub).toBe('alice')
    const second = await s.consume('h1', new Date())
    expect(second).toBeNull()
  })

  it('期限切れは consume で null になり、行は削除される', async () => {
    const s = new InMemoryPasswordResetTokenStore()
    const now = new Date('2026-06-13T12:00:00Z')
    await s.save(record('alice', 'h1', 60, now))
    const later = new Date(now.getTime() + 120 * 1000)
    expect(await s.consume('h1', later)).toBeNull()
    // 削除済みなので二度目も null
    expect(await s.consume('h1', later)).toBeNull()
  })

  it('未存在 hash は null', async () => {
    const s = new InMemoryPasswordResetTokenStore()
    expect(await s.consume('missing', new Date())).toBeNull()
  })

  it('同 sub の新規 save は旧 token を失効させる', async () => {
    const s = new InMemoryPasswordResetTokenStore()
    await s.save(record('alice', 'old'))
    await s.save(record('alice', 'new'))
    expect(await s.consume('old', new Date())).toBeNull()
    expect((await s.consume('new', new Date()))?.sub).toBe('alice')
  })

  it('別 sub は独立に保管される', async () => {
    const s = new InMemoryPasswordResetTokenStore()
    await s.save(record('alice', 'h1'))
    await s.save(record('bob', 'h2'))
    expect((await s.consume('h1', new Date()))?.sub).toBe('alice')
    expect((await s.consume('h2', new Date()))?.sub).toBe('bob')
  })
})
