/**
 * Property: L4 — DPoP jti uniqueness within window (spec/scl.yaml invariants.DpopJtiUniquenessWithinWindow)
 */

import { describe, it } from 'bun:test'
import fc from 'fast-check'
import { InMemoryDpopReplayStore } from '../../../adapters/persistence/memory/dpop-replay-store'

describe('L4 — DPoP jti uniqueness property', () => {
  it('同じ jti は recordIfNew で 2 度 true を返さない', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.string({ minLength: 8, maxLength: 64 }),
        fc.integer({ min: 60, max: 600 }),
        async (jti, windowSec) => {
          const store = new InMemoryDpopReplayStore()
          const first = await store.recordIfNew(jti, windowSec)
          const second = await store.recordIfNew(jti, windowSec)
          return first === true && second === false
        },
      ),
      { numRuns: 100 },
    )
  })

  it('異なる jti は両方 true', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.string({ minLength: 8, maxLength: 64 }),
        fc.string({ minLength: 8, maxLength: 64 }),
        async (a, b) => {
          if (a === b) return true
          const store = new InMemoryDpopReplayStore()
          const ra = await store.recordIfNew(a, 600)
          const rb = await store.recordIfNew(b, 600)
          return ra === true && rb === true
        },
      ),
      { numRuns: 50 },
    )
  })
})
