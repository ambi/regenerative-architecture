/**
 * Layer 3 — Application Logic（ドメイン単体テスト）
 *
 * RFC 7636 PKCE 検証ロジックのプロパティテスト。
 */

import { describe, it, expect } from 'bun:test'
import { createHash, randomBytes } from 'crypto'
import fc from 'fast-check'
import { verifyPkce } from './pkce'

function challengeOf(verifier: string): string {
  return createHash('sha256').update(verifier).digest('base64url')
}

describe('verifyPkce', () => {
  it('正しい verifier / challenge ペアを受け入れる', () => {
    const verifier = randomBytes(32).toString('base64url')
    expect(verifyPkce(verifier, challengeOf(verifier))).toBe(true)
  })

  it('不正な verifier を拒否する', () => {
    const verifier = randomBytes(32).toString('base64url')
    const wrong = randomBytes(32).toString('base64url')
    expect(verifyPkce(wrong, challengeOf(verifier))).toBe(false)
  })

  it('challenge と長さが異なる入力を拒否する（短絡判定で timing-leak 防止）', () => {
    const verifier = 'verifier-1'
    expect(verifyPkce(verifier, 'too-short')).toBe(false)
  })

  it('S256 以外のメソッドを拒否する', () => {
    const verifier = 'verifier-1'
    // @ts-expect-error — 仕様外メソッドは型レベルでも拒否
    expect(verifyPkce(verifier, challengeOf(verifier), 'plain')).toBe(false)
  })

  it('property: SHA-256 で計算した challenge は常に検証成功する', () => {
    fc.assert(
      fc.property(fc.stringMatching(/^[A-Za-z0-9_-]{43,128}$/), (verifier: string) => {
        return verifyPkce(verifier, challengeOf(verifier)) === true
      }),
    )
  })

  it('property: verifier の 1 ビット変化は検証を必ず失敗させる', () => {
    fc.assert(
      fc.property(
        fc.stringMatching(/^[A-Za-z0-9_-]{43,128}$/),
        fc.integer({ min: 0, max: 42 }),
        (verifier: string, idx: number) => {
          const ch = challengeOf(verifier)
          // verifier の 1 文字を別の文字に置き換える
          const replacement = verifier[idx] === 'A' ? 'B' : 'A'
          const tampered = verifier.slice(0, idx) + replacement + verifier.slice(idx + 1)
          return verifyPkce(tampered, ch) === false
        },
      ),
    )
  })
})
