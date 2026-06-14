/**
 * Layer 3 — SCL boundary checks
 *
 * 実装ファイルやテスト名との対応は SCL ではなく、実装側の assurance manifest で管理する。
 */

import { describe, expect, it } from 'bun:test'
import { scl } from './scl'

describe('SCL implementation boundary', () => {
  it('top-level annotations に実装追跡情報を持たない', () => {
    expect(scl.annotations?.scenario_coverage).toBeUndefined()
    expect(JSON.stringify(scl)).not.toContain('"implementation"')
  })

  it('scenario は規範的な受け入れ条件だけを保持する', () => {
    expect(Object.keys(scl.scenarios).length).toBeGreaterThan(0)
    for (const scenario of Object.values(scl.scenarios)) {
      expect(scenario.steps.length).toBeGreaterThan(0)
    }
  })
})
