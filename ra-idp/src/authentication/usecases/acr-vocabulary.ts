/**
 * Layer 3 — Application Logic (Authentication Context Class Reference)
 *
 * 仕様核: spec/scl.yaml annotations.acr_vocabulary。本ファイルは SCL と双子定義であり、
 * 値が乖離すると spec↔impl drift になるため check:coherence で突き合わせる。
 *
 * OIDC Core §2 で acr の値域は AS が自由に定められるため、本 IdP が発行する URN を
 * ここで固定し、amr (RFC 8176) からの導出を pure 関数として提供する。step-up reauth
 * (acr_values) もこの URN を介して評価される。
 */

export const ACR_VALUES = {
  /** Password 単独認証 (amr が [pwd] のみ、または MFA factor を含まない)。 */
  pwd: 'urn:ra-idp:acr:pwd',
  /** Password + 第二要素 (amr に otp / webauthn / hwk / swk のいずれかを含む)。 */
  mfa: 'urn:ra-idp:acr:mfa',
} as const

export type AcrValue = (typeof ACR_VALUES)[keyof typeof ACR_VALUES]

/** SCL annotations.acr_vocabulary.mfa_amr_values と一致させる。 */
const MFA_AMR_VALUES: readonly string[] = ['otp', 'webauthn', 'hwk', 'swk']

export function deriveAcr(amr: readonly string[]): AcrValue {
  return amr.some((m) => MFA_AMR_VALUES.includes(m)) ? ACR_VALUES.mfa : ACR_VALUES.pwd
}

/**
 * acr_values (RFC 6749 §3.1.2 / OIDC Core §3.1.2.1) は空白区切りの URN 列。
 * 現セッションの acr が要求された URN のいずれかを満たすかを判定する。
 *
 * 「満たす」の定義: mfa は pwd を包含するが、pwd は mfa を満たさない。
 * 要求が空白区切りで複数与えられた場合、いずれか 1 つを満たせば OK。
 */
export function acrSatisfies(currentAcr: string, requested: string): boolean {
  const requestedList = requested.split(/\s+/).filter(Boolean) as AcrValue[]
  return requestedList.some((r) => {
    if (r === currentAcr) return true
    // mfa は pwd を包含する: 現 acr=mfa は要求 pwd を満たす
    if (currentAcr === ACR_VALUES.mfa && r === ACR_VALUES.pwd) return true
    return false
  })
}
