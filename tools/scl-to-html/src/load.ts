/**
 * Filesystem loaders for the four input kinds:
 *
 *   - SCL document   — single YAML file (Bun's native YAML importer)
 *   - Decisions      — directory of *.md (CONCEPTION*.md + ADR-*.md)
 *   - Work items     — directory of *.yaml
 *
 * Pure-ish: file IO only. No network, no clock.
 */

import { readFile, readdir } from 'node:fs/promises'
import { basename, extname, join } from 'node:path'
import { pathToFileURL } from 'node:url'
import { splitTitle } from './markdown.ts'
import type { ChangeEntry, DecisionDoc, SclDocument, WorkItem } from './types.ts'

export async function loadScl(path: string): Promise<SclDocument> {
  const mod = await import(pathToFileURL(path).href)
  const data = (mod as { default?: unknown }).default ?? mod
  if (!data || typeof data !== 'object' || Array.isArray(data)) {
    throw new Error(`SCL document ${path} did not parse to an object`)
  }
  return data as SclDocument
}

const ADR_FILENAME_RE = /^ADR-(\d{1,4})-.+\.md$/i
const CONCEPTION_FILENAME_RE = /^CONCEPTION(?:_[A-Z]+)?\.md$/

export async function loadDecisions(dir: string): Promise<DecisionDoc[]> {
  const names = await readdir(dir)
  const wanted = names
    .filter((n) => CONCEPTION_FILENAME_RE.test(n) || ADR_FILENAME_RE.test(n))
    .sort()
  const out: DecisionDoc[] = []
  for (const name of wanted) {
    const path = join(dir, name)
    const source = await readFile(path, 'utf8')
    const isConception = CONCEPTION_FILENAME_RE.test(name)
    const adrMatch = name.match(ADR_FILENAME_RE)
    const id = isConception
      ? name.toLowerCase().replace(/\.md$/, '').replace(/_/g, '-')
      : name.toLowerCase().replace(/\.md$/, '')
    const { title, body } = splitTitle(source, basename(name, '.md'))
    out.push({
      id,
      title,
      kind: isConception ? 'conception' : 'adr',
      filename: name,
      body,
      number: adrMatch ? Number.parseInt(adrMatch[1] ?? '0', 10) : undefined,
    })
  }
  return out
}

export async function loadChanges(dir: string): Promise<ChangeEntry[]> {
  const entries = await readdir(dir, { withFileTypes: true })
  const files = entries
    .filter((e) => e.isFile() && extname(e.name) === '.yaml')
    .map((e) => e.name)
    .sort()
  const out: ChangeEntry[] = []
  for (const file of files) {
    const id = basename(file, '.yaml')
    const wiPath = join(dir, file)
    let work_item: WorkItem
    try {
      const mod = await import(pathToFileURL(wiPath).href)
      const data = (mod as { default?: unknown }).default
      if (!data || typeof data !== 'object' || Array.isArray(data)) continue
      work_item = { id, ...(data as object) } as WorkItem
    } catch {
      // Failed to parse — skip this file.
      continue
    }
    out.push({ id, work_item })
  }
  return out
}
