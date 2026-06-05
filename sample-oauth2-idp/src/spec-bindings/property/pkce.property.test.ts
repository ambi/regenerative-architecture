/**
 * Property: L1 — PKCE round-trip (spec/scl.yaml properties.PkceRoundTrip)
 *
 * 任意の verifier ∈ RFC 7636 で
 *   challenge = base64url(SHA256(verifier))
 *   verifyPkce(verifier, challenge, 'S256') === true
 */

import { describe, it } from 'bun:test'
import fc from 'fast-check'
import { createHash } from 'crypto'
import { verifyPkce } from '../../domain/pkce'

// RFC 7636 §4.1 の文字集合: ALPHA / DIGIT / "-" / "." / "_" / "~"
const PKCE_CHAR_REGEX = /^[A-Za-z0-9\-._~]+$/

describe('L1 — PKCE round-trip property', () => {
  it('任意の verifier (43-128 chars) で S256 が round-trip する', () => {
    fc.assert(
      fc.property(fc.stringMatching(/^[A-Za-z0-9\-._~]{43,128}$/), (verifier) => {
        if (!PKCE_CHAR_REGEX.test(verifier)) return true
        if (verifier.length < 43 || verifier.length > 128) return true
        const challenge = createHash('sha256').update(verifier).digest('base64url')
        return verifyPkce(verifier, challenge, 'S256') === true
      }),
      { numRuns: 200 },
    )
  })

  it('壊れた challenge では false (反例反転テスト)', () => {
    fc.assert(
      fc.property(
        fc.stringMatching(/^[A-Za-z0-9\-._~]{43,128}$/),
        fc.stringMatching(/^[A-Za-z0-9\-._~]{43,128}$/),
        (a, b) => {
          if (a === b) return true
          const challenge = createHash('sha256').update(a).digest('base64url')
          // verifier b は challenge と関係ないので false が期待される
          return verifyPkce(b, challenge, 'S256') === false
        },
      ),
      { numRuns: 100 },
    )
  })
})
