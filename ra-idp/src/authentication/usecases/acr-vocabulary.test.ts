import { describe, expect, it } from 'bun:test'
import { ACR_VALUES, acrSatisfies, deriveAcr } from './acr-vocabulary'

describe('deriveAcr', () => {
  it('amr=[pwd] のみは pwd', () => {
    expect(deriveAcr(['pwd'])).toBe(ACR_VALUES.pwd)
  })

  it('amr が空でも pwd (MFA factor を含まないため)', () => {
    expect(deriveAcr([])).toBe(ACR_VALUES.pwd)
  })

  it('amr に otp を含めば mfa', () => {
    expect(deriveAcr(['pwd', 'otp'])).toBe(ACR_VALUES.mfa)
  })

  it('amr に webauthn を含めば mfa', () => {
    expect(deriveAcr(['pwd', 'webauthn'])).toBe(ACR_VALUES.mfa)
  })

  it('未知の amr は MFA に昇格しない', () => {
    expect(deriveAcr(['pwd', 'unknown_factor'])).toBe(ACR_VALUES.pwd)
  })
})

describe('acrSatisfies', () => {
  it('同じ URN は満たす', () => {
    expect(acrSatisfies(ACR_VALUES.pwd, ACR_VALUES.pwd)).toBe(true)
    expect(acrSatisfies(ACR_VALUES.mfa, ACR_VALUES.mfa)).toBe(true)
  })

  it('mfa は pwd を包含する', () => {
    expect(acrSatisfies(ACR_VALUES.mfa, ACR_VALUES.pwd)).toBe(true)
  })

  it('pwd は mfa を満たさない', () => {
    expect(acrSatisfies(ACR_VALUES.pwd, ACR_VALUES.mfa)).toBe(false)
  })

  it('空白区切りで OR 評価される', () => {
    expect(acrSatisfies(ACR_VALUES.pwd, `${ACR_VALUES.mfa} ${ACR_VALUES.pwd}`)).toBe(true)
    expect(acrSatisfies(ACR_VALUES.pwd, `${ACR_VALUES.mfa}`)).toBe(false)
  })
})
