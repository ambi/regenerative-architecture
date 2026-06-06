import { describe, expect, it } from 'bun:test'

import { PASSWORD_POLICY, validatePassword } from './password-policy'

describe('password policy', () => {
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
