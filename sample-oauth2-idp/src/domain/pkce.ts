/**
 * Layer 3 — Application Logic
 *
 * PKCE (RFC 7636) 検証ロジック。
 * 仕様核では「SHA-256 の S256 method」のみを許容する（ADR-002）。
 */

import { createHash, timingSafeEqual } from 'crypto'

/**
 * 与えられた code_verifier が code_challenge と一致するか検証する。
 * 一定時間比較でタイミング攻撃を防ぐ。
 */
export function verifyPkce(verifier: string, challenge: string, method: 'S256' = 'S256'): boolean {
  if (method !== 'S256') return false
  const computed = createHash('sha256').update(verifier).digest('base64url')
  if (computed.length !== challenge.length) return false
  return timingSafeEqual(Buffer.from(computed), Buffer.from(challenge))
}
