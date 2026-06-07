import { describe, expect, it } from 'bun:test'
import { buildOtpauthUri, generateTotp, generateTotpSecret, verifyTotp } from './totp'

// RFC 6238 Appendix B: SHA-1 用テスト秘密 "12345678901234567890" (ASCII 20 byte)
// の base32 表現。実装の base32Encode の round-trip と独立して
// アルゴリズム自体を検証するため、この既知ベクトルを定数で持つ。
const RFC6238_SHA1_SECRET_B32 = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'

describe('generateTotp (RFC 6238 Appendix B / SHA-1 vectors)', () => {
  const cases: Array<[number, string]> = [
    [59, '94287082'.slice(-6)],
    [1111111109, '07081804'.slice(-6)],
    [1111111111, '14050471'.slice(-6)],
    [1234567890, '89005924'.slice(-6)],
    [2000000000, '69279037'.slice(-6)],
    [20000000000, '65353130'.slice(-6)],
  ]

  for (const [t, expected] of cases) {
    it(`T=${t} → ${expected}`, () => {
      expect(generateTotp(RFC6238_SHA1_SECRET_B32, t)).toBe(expected)
    })
  }
})

describe('verifyTotp', () => {
  it('現在ステップのコードを受理する', () => {
    const t = 1700000000
    const code = generateTotp(RFC6238_SHA1_SECRET_B32, t)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, code, t)).toBe(true)
  })

  it('window=1 で前後 1 ステップずれを許容する', () => {
    const t = 1700000000
    const prev = generateTotp(RFC6238_SHA1_SECRET_B32, t - 30)
    const next = generateTotp(RFC6238_SHA1_SECRET_B32, t + 30)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, prev, t)).toBe(true)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, next, t)).toBe(true)
  })

  it('window=1 でも 2 ステップ離れたコードは拒否する', () => {
    const t = 1700000000
    const tooOld = generateTotp(RFC6238_SHA1_SECRET_B32, t - 60)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, tooOld, t)).toBe(false)
  })

  it('長さ不正・非数字は拒否する', () => {
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, '12345', 0)).toBe(false)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, '1234567', 0)).toBe(false)
    expect(verifyTotp(RFC6238_SHA1_SECRET_B32, 'abc123', 0)).toBe(false)
  })
})

describe('generateTotpSecret', () => {
  it('base32 文字列 (160-bit) を返す', () => {
    const secret = generateTotpSecret()
    expect(secret).toMatch(/^[A-Z2-7]{32}$/)
  })

  it('2 回の呼び出しは別の値を返す', () => {
    expect(generateTotpSecret()).not.toBe(generateTotpSecret())
  })
})

describe('buildOtpauthUri', () => {
  it('otpauth://totp/{issuer}:{account}?secret=...&issuer=... 形式', () => {
    const uri = buildOtpauthUri({
      secretBase32: RFC6238_SHA1_SECRET_B32,
      accountName: 'alice@example.com',
      issuer: 'RA IdP',
    })
    expect(uri).toContain('otpauth://totp/')
    expect(uri).toContain(`secret=${RFC6238_SHA1_SECRET_B32}`)
    expect(uri).toContain('issuer=RA+IdP')
    expect(uri).toContain('algorithm=SHA1')
    expect(uri).toContain('digits=6')
    expect(uri).toContain('period=30')
  })
})
