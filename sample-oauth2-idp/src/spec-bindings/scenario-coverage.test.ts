/**
 * Layer 3 — Scenario coverage binding
 *
 * SCL scenarios は保存対象で、実行テスト・手動 smoke・部分検証は派生物。
 * このテストは「SCL にシナリオを追加したのに coverage matrix を更新し忘れる」
 * というドリフトを検出する。
 */

import { describe, expect, it } from 'bun:test'
import { scl } from './scl'

type CoverageStatus = 'covered' | 'partial' | 'manual' | 'missing'

type ScenarioCoverage = {
  status: CoverageStatus
  evidence?: Array<{ file: string; test?: string }>
  note?: string
}

function scenarioCoverage(): Record<string, ScenarioCoverage> {
  return (scl.annotations?.scenario_coverage ?? {}) as Record<string, ScenarioCoverage>
}

describe('SCL scenarios — coverage matrix', () => {
  it('すべての SCL scenario が coverage matrix に分類されている', () => {
    const scenarios = Object.keys(scl.scenarios).sort()
    const coverage = Object.keys(scenarioCoverage()).sort()
    expect(coverage).toEqual(scenarios)
  })

  it('missing のままの scenario がない', () => {
    const missing = Object.entries(scenarioCoverage())
      .filter(([, entry]) => entry.status === 'missing')
      .map(([name]) => name)
    expect(missing).toEqual([])
  })

  it('covered / partial / manual は evidence を持つ', () => {
    for (const [name, entry] of Object.entries(scenarioCoverage())) {
      if (entry.status === 'missing') continue
      expect(`${name}: evidence`).toBe(`${name}: evidence`)
      expect(entry.evidence?.length ?? 0).toBeGreaterThan(0)
    }
  })
})
