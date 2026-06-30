/**
 * Semantic verification for the SCL `context_map` section.
 *
 * The JSON Schema (`schemas/scl.schema.json`) only checks the *shape* of
 * context_map entries (the `via` enum, that `uses` is an array, etc.). It
 * cannot check meaning across entries. This module adds the cross-entry
 * checks that catch real bounded-context modelling mistakes:
 *
 *   1. reference resolution (error) — every `depends_on.<Target>` names an
 *      existing context, and every name in `uses` is actually `publishes`-ed
 *      by that target.
 *   2. acyclic dependencies (error) — `depends_on` forms a DAG.
 *   3. shared_kernel size (warning) — a `via: shared_kernel` relationship that
 *      shares many names is an anti-pattern; suggest a narrower integration.
 *
 * Everything here is pure. Line numbers are resolved with `locatePointer` so
 * findings point at the offending key in the source text, like the schema
 * validator does.
 */

import { type Finding, locatePointer } from './lib.ts'

/** A shared_kernel relationship sharing more names than this is flagged. */
export const SHARED_KERNEL_MAX = 3

export type ContextMapReport = { errors: Finding[]; warnings: Finding[] }

type Relation = { uses?: unknown; via?: unknown; reason?: unknown }
type ContextEntry = { publishes?: unknown; depends_on?: unknown }

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}

export function verifyContextMap(doc: unknown, text = ''): ContextMapReport {
  const errors: Finding[] = []
  const warnings: Finding[] = []

  const cm = (doc as { context_map?: unknown } | null | undefined)?.context_map
  if (cm === undefined || cm === null || typeof cm !== 'object') return { errors, warnings }

  const contexts = cm as Record<string, unknown>
  const names = new Set(Object.keys(contexts))

  // name -> set of published names
  const published = new Map<string, Set<string>>()
  for (const [name, entryU] of Object.entries(contexts)) {
    const entry = entryU as ContextEntry
    published.set(name, new Set(asStringArray(entry?.publishes)))
  }

  // name -> list of context names it depends on (for cycle detection)
  const edges = new Map<string, string[]>()

  for (const [name, entryU] of Object.entries(contexts)) {
    const entry = entryU as ContextEntry
    const dependsOn = entry?.depends_on
    const targets: string[] = []
    if (dependsOn && typeof dependsOn === 'object') {
      for (const [target, relU] of Object.entries(dependsOn as Record<string, unknown>)) {
        const ptr = `/context_map/${name}/depends_on/${target}`
        if (!names.has(target)) {
          errors.push({
            line: locatePointer(text, ptr),
            column: 1,
            message: `context-map: ${name}.depends_on references unknown context '${target}'`,
          })
          continue
        }
        targets.push(target)
        const rel = relU as Relation
        const uses = asStringArray(rel?.uses)
        const targetPublished = published.get(target) ?? new Set<string>()
        uses.forEach((used, i) => {
          if (!targetPublished.has(used)) {
            errors.push({
              line: locatePointer(text, `${ptr}/uses/${i}`),
              column: 1,
              message: `context-map: ${name} uses '${used}' which '${target}' does not publish`,
            })
          }
        })
        if (rel?.via === 'shared_kernel' && uses.length > SHARED_KERNEL_MAX) {
          warnings.push({
            line: locatePointer(text, ptr),
            column: 1,
            message: `context-map (warning): shared_kernel ${name} -> ${target} shares ${uses.length} names (> ${SHARED_KERNEL_MAX}); consider published_language or anticorruption_layer`,
          })
        }
      }
    }
    edges.set(name, targets)
  }

  const cycle = findCycle(edges)
  if (cycle) {
    errors.push({
      line: locatePointer(text, `/context_map/${cycle[0]}`),
      column: 1,
      message: `context-map: dependency cycle detected: ${cycle.join(' -> ')}`,
    })
  }

  return { errors, warnings }
}

/**
 * Return one dependency cycle as a node path (first node repeated at the end),
 * or null when the graph is acyclic. Standard white/gray/black DFS.
 */
function findCycle(edges: Map<string, string[]>): string[] | null {
  const WHITE = 0
  const GRAY = 1
  const BLACK = 2
  const color = new Map<string, number>()
  const stack: string[] = []
  let found: string[] | null = null

  const visit = (node: string): boolean => {
    color.set(node, GRAY)
    stack.push(node)
    for (const next of edges.get(node) ?? []) {
      const c = color.get(next) ?? WHITE
      if (c === GRAY) {
        const idx = stack.indexOf(next)
        found = [...stack.slice(idx), next]
        return true
      }
      if (c === WHITE && visit(next)) return true
    }
    stack.pop()
    color.set(node, BLACK)
    return false
  }

  for (const node of edges.keys()) {
    if ((color.get(node) ?? WHITE) === WHITE && visit(node)) break
  }
  return found
}
