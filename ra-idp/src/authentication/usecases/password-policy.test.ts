import { describe, expect, it } from 'bun:test'

import { COMMON_PASSWORDS } from './common-passwords'
import { PASSWORD_POLICY, validatePassword } from './password-policy'

describe('password policy — 長さ', () => {
  it('accepts a password at the minimum length', () => {
    const min = 'x'.repeat(PASSWORD_POLICY.minLength)
    expect(validatePassword(min)).toEqual({ ok: true })
  })

  it('rejects a password shorter than the minimum length', () => {
    const short = 'x'.repeat(PASSWORD_POLICY.minLength - 1)
    expect(validatePassword(short)).toEqual({ ok: false, violations: ['too_short'] })
  })

  it('accepts a password at the maximum length', () => {
    const max = 'x'.repeat(PASSWORD_POLICY.maxLength)
    expect(validatePassword(max)).toEqual({ ok: true })
  })

  it('rejects a password longer than the maximum length', () => {
    const over = 'x'.repeat(PASSWORD_POLICY.maxLength + 1)
    expect(validatePassword(over)).toEqual({ ok: false, violations: ['too_long'] })
  })

  it('rejects an empty password', () => {
    expect(validatePassword('')).toEqual({ ok: false, violations: ['too_short'] })
  })
})

describe('password policy — ユーザー識別子との類似', () => {
  it('rejects a password that contains the username', () => {
    const result = validatePassword('alice-foobar-92', { username: 'alice' })
    expect(result).toEqual({ ok: false, violations: ['similar_to_identifier'] })
  })

  it('rejects case-insensitively', () => {
    const result = validatePassword('Alice-foobar-92', { username: 'alice' })
    expect(result).toEqual({ ok: false, violations: ['similar_to_identifier'] })
  })

  it('rejects a password that contains the email local-part', () => {
    const result = validatePassword('bobby-secret-92', { email: 'bobby@example.com' })
    expect(result).toEqual({ ok: false, violations: ['similar_to_identifier'] })
  })

  it('rejects a password that contains the full email', () => {
    const result = validatePassword('alice@example.com-x9', { email: 'alice@example.com' })
    expect(result).toEqual({ ok: false, violations: ['similar_to_identifier'] })
  })

  it('skips similarity check for very short identifiers', () => {
    // 識別子が 4 文字未満なら誤検知を避けるためチェックしない
    const result = validatePassword('safelong-passw0rd', { username: 'ab' })
    expect(result).toEqual({ ok: true })
  })

  it('passes when identifier does not appear in the password', () => {
    const result = validatePassword('unrelated-strong-91', { username: 'alice' })
    expect(result).toEqual({ ok: true })
  })

  it('accepts when context is omitted', () => {
    expect(validatePassword('alice-foobar-92')).toEqual({ ok: true })
  })
})

describe('password policy — 共通パスワード辞書', () => {
  it('rejects a bundled common password', () => {
    const sample = 'password1234' // bundled list に含まれる
    expect(COMMON_PASSWORDS.has(sample)).toBe(true)
    expect(validatePassword(sample)).toEqual({ ok: false, violations: ['common_password'] })
  })

  it('rejects case-insensitively', () => {
    expect(validatePassword('PASSWORD1234')).toEqual({
      ok: false,
      violations: ['common_password'],
    })
  })

  it('passes for a non-dictionary password of sufficient length', () => {
    expect(validatePassword('correct-horse-battery')).toEqual({ ok: true })
  })
})

describe('password policy — 複数違反', () => {
  it('reports all violations together', () => {
    // 短い + 辞書ヒット
    const result = validatePassword('password', { username: 'password' })
    expect(result.ok).toBe(false)
    if (!result.ok) {
      expect(result.violations).toEqual(
        expect.arrayContaining(['too_short', 'similar_to_identifier', 'common_password']),
      )
    }
  })
})
