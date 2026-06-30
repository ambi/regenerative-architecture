import { describe, expect, it } from 'bun:test'
import { SHARED_KERNEL_MAX, verifyContextMap } from './context-map.ts'

const base = (contextMap: Record<string, unknown>) => ({
  system: 'demo',
  spec_version: '2.0',
  context_map: contextMap,
})

describe('verifyContextMap', () => {
  it('is a no-op when no context_map is present', () => {
    expect(verifyContextMap({ system: 'demo', spec_version: '2.0' })).toEqual({
      errors: [],
      warnings: [],
    })
  })

  it('accepts a well-formed acyclic map', () => {
    const doc = base({
      Tenancy: { description: 't', publishes: ['TenantRef'] },
      Identity: {
        description: 'i',
        publishes: ['UserRef'],
        depends_on: { Tenancy: { via: 'published_language', uses: ['TenantRef'] } },
      },
    })
    expect(verifyContextMap(doc)).toEqual({ errors: [], warnings: [] })
  })

  it('flags a depends_on target that does not exist', () => {
    const doc = base({
      Identity: {
        description: 'i',
        depends_on: { Ghost: { via: 'published_language', uses: ['X'] } },
      },
    })
    const { errors } = verifyContextMap(doc)
    expect(errors.some((e) => e.message.includes("unknown context 'Ghost'"))).toBe(true)
  })

  it('flags a uses name not published by the target', () => {
    const doc = base({
      Tenancy: { description: 't', publishes: ['TenantRef'] },
      Identity: {
        description: 'i',
        depends_on: { Tenancy: { via: 'published_language', uses: ['NotPublished'] } },
      },
    })
    const { errors } = verifyContextMap(doc)
    expect(
      errors.some((e) =>
        e.message.includes("uses 'NotPublished' which 'Tenancy' does not publish"),
      ),
    ).toBe(true)
  })

  it('detects a dependency cycle', () => {
    const doc = base({
      A: { description: 'a', publishes: ['Ax'], depends_on: { B: { uses: ['Bx'] } } },
      B: { description: 'b', publishes: ['Bx'], depends_on: { A: { uses: ['Ax'] } } },
    })
    const { errors } = verifyContextMap(doc)
    expect(errors.some((e) => e.message.includes('dependency cycle detected'))).toBe(true)
  })

  it('accepts a self-published name used across a long acyclic chain', () => {
    const doc = base({
      A: { description: 'a', publishes: ['Ax'] },
      B: { description: 'b', publishes: ['Bx'], depends_on: { A: { uses: ['Ax'] } } },
      C: { description: 'c', depends_on: { B: { uses: ['Bx'] }, A: { uses: ['Ax'] } } },
    })
    expect(verifyContextMap(doc).errors).toEqual([])
  })

  it('warns (but does not error) on an oversized shared_kernel', () => {
    const shared = Array.from({ length: SHARED_KERNEL_MAX + 1 }, (_, i) => `N${i}`)
    const doc = base({
      Core: { description: 'c', publishes: shared },
      Other: {
        description: 'o',
        depends_on: { Core: { via: 'shared_kernel', uses: shared } },
      },
    })
    const { errors, warnings } = verifyContextMap(doc)
    expect(errors).toEqual([])
    expect(warnings.some((w) => w.message.includes('shared_kernel'))).toBe(true)
  })

  it('does not warn on a shared_kernel at or below the threshold', () => {
    const shared = Array.from({ length: SHARED_KERNEL_MAX }, (_, i) => `N${i}`)
    const doc = base({
      Core: { description: 'c', publishes: shared },
      Other: {
        description: 'o',
        depends_on: { Core: { via: 'shared_kernel', uses: shared } },
      },
    })
    expect(verifyContextMap(doc).warnings).toEqual([])
  })
})
