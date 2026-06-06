/**
 * Layer 4 — Adapter test: HMAC DPoP-Nonce サービス。
 */

import { describe, expect, it } from 'bun:test'

import { HmacDpopNonceService } from './hmac-dpop-nonce-service'

describe('HmacDpopNonceService', () => {
  it('issue した nonce は verify を通る', () => {
    const svc = HmacDpopNonceService.withRandomSecret(60)
    const nonce = svc.issue()
    expect(svc.verify(nonce)).toBe(true)
  })

  it('TTL を超えた nonce は拒否される', () => {
    const svc = HmacDpopNonceService.withRandomSecret(10)
    const issuedAt = new Date('2026-01-01T00:00:00Z')
    const nonce = svc.issue(issuedAt)
    const later = new Date(issuedAt.getTime() + 20_000)
    expect(svc.verify(nonce, later)).toBe(false)
  })

  it('改ざんされた nonce は MAC 不一致で拒否される', () => {
    const svc = HmacDpopNonceService.withRandomSecret(60)
    const nonce = svc.issue()
    const tampered = `${nonce.slice(0, -2)}AA`
    expect(svc.verify(tampered)).toBe(false)
  })

  it('別のサービス (別 secret) で発行した nonce は拒否される', () => {
    const a = HmacDpopNonceService.withRandomSecret(60)
    const b = HmacDpopNonceService.withRandomSecret(60)
    expect(b.verify(a.issue())).toBe(false)
  })

  it('不正な形式の nonce は安全に false を返す', () => {
    const svc = HmacDpopNonceService.withRandomSecret(60)
    expect(svc.verify('not-a-real-nonce')).toBe(false)
    expect(svc.verify('')).toBe(false)
  })
})
