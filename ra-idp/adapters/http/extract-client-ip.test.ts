/**
 * Layer 4 — Adapter Layer (extractClientIp テスト)
 */

import { describe, expect, it } from 'bun:test'

import { extractClientIp } from './extract-client-ip'

function headersOf(init: Record<string, string>): Headers {
  return new Headers(init)
}

describe('extractClientIp', () => {
  it('trustedHops=0 は X-Forwarded-For があっても null', () => {
    const h = headersOf({ 'x-forwarded-for': '1.2.3.4' })
    expect(extractClientIp(h, { trustedHops: 0 })).toBeNull()
  })

  it('X-Forwarded-For 無しは null', () => {
    expect(extractClientIp(headersOf({}), { trustedHops: 1 })).toBeNull()
  })

  it('trustedHops=1 で 1 段プロキシ越しの client IP を採用', () => {
    // chain: client → proxy → us。XFF は client が左、proxy が無記入 (proxy 自身は
    // 直接 peer なので付け足さない)。つまり XFF = "client"。trustedHops=1 で
    // 右端から 1 つ skip した上で最後に残るのが client。
    // 実際の Nginx は trust_proxy=1 のとき XFF の rightmost-1 を client IP とするので
    // ここでは len=1, trustedHops=1 → idx = 0 - 1 = -1 → null (XFF が空に近い)。
    // 1 段プロキシで実際に入る XFF は "client_ip" の 1 entry。これを採用するには
    // trustedHops=0 で peer_addr を使うか、trustedHops=1 + XFF len>=2 が必要。
    // 本テストは「rightmost-N が client」のルールが守られていることを確認する。
    const xff = '1.1.1.1, 10.0.0.1'
    expect(extractClientIp(headersOf({ 'x-forwarded-for': xff }), { trustedHops: 1 })).toBe(
      '1.1.1.1',
    )
  })

  it('trustedHops=2 で 2 段プロキシ越しの client IP を採用', () => {
    const xff = '1.1.1.1, 10.0.0.1, 10.0.0.2'
    expect(extractClientIp(headersOf({ 'x-forwarded-for': xff }), { trustedHops: 2 })).toBe(
      '1.1.1.1',
    )
  })

  it('trustedHops が XFF 段数より多ければ null', () => {
    const xff = '1.1.1.1'
    expect(extractClientIp(headersOf({ 'x-forwarded-for': xff }), { trustedHops: 2 })).toBeNull()
  })

  it('空白を含む XFF も正しく分割される', () => {
    const xff = '  1.1.1.1  ,  10.0.0.1  '
    expect(extractClientIp(headersOf({ 'x-forwarded-for': xff }), { trustedHops: 1 })).toBe(
      '1.1.1.1',
    )
  })
})
