/**
 * End-to-end loader tests against ephemeral fixtures in tmpdir.
 *
 * Bun's dynamic import caches by URL, so each fixture gets a unique
 * directory path to avoid stale results across runs.
 */

import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, describe, expect, it } from 'bun:test'
import { loadChanges, loadDecisions, loadScl, loadSclBundle } from './load.ts'

const cleanup: string[] = []
afterAll(async () => {
  for (const d of cleanup) {
    await rm(d, { recursive: true, force: true })
  }
})

const tempDir = async (): Promise<string> => {
  const dir = await mkdtemp(join(tmpdir(), 'scl-to-html-test-'))
  cleanup.push(dir)
  return dir
}

describe('loadScl', () => {
  it('reads a YAML SCL document via Bun YAML import', async () => {
    const dir = await tempDir()
    const path = join(dir, 'scl.yaml')
    await writeFile(path, 'system: demo\nspec_version: "1.0"\n')
    const doc = await loadScl(path)
    expect(doc.system).toBe('demo')
    expect(doc.spec_version).toBe('1.0')
  })

  it('throws if the file does not parse to an object', async () => {
    const dir = await tempDir()
    const path = join(dir, 'scl.yaml')
    await writeFile(path, '- a\n- b\n')
    await expect(loadScl(path)).rejects.toThrow()
  })
})

describe('loadSclBundle', () => {
  it('loads context documents referenced by root context_map paths', async () => {
    const dir = await tempDir()
    await mkdir(join(dir, 'contexts'))
    await writeFile(
      join(dir, 'scl.yaml'),
      [
        'system: demo',
        'spec_version: "2.0"',
        'context_map:',
        '  App:',
        '    path: contexts/application.yaml',
      ].join('\n') + '\n',
    )
    await writeFile(
      join(dir, 'contexts', 'application.yaml'),
      [
        'system: demo',
        'spec_version: "2.0"',
        'context: Application',
        'models:',
        '  App:',
        '    kind: entity',
        '    identity: id',
        '    fields:',
        '      id: { type: UUID }',
      ].join('\n') + '\n',
    )

    const bundle = await loadSclBundle(join(dir, 'scl.yaml'))
    expect(bundle.root.system).toBe('demo')
    expect(bundle.contexts).toHaveLength(1)
    expect(bundle.contexts[0]?.name).toBe('App')
    expect(bundle.contexts[0]?.path).toBe('contexts/application.yaml')
    expect(bundle.contexts[0]?.document.context).toBe('Application')
  })
})

describe('loadDecisions', () => {
  it('parses CONCEPTION + ADR-N markdown files', async () => {
    const dir = await tempDir()
    await writeFile(join(dir, 'CONCEPTION.md'), '# Conception\n\nbody\n')
    await writeFile(join(dir, 'CONCEPTION_BASELINE.md'), '# Baseline\n\nbody\n')
    await writeFile(join(dir, 'ADR-001-foo.md'), '# ADR-001: Foo\n\nbody\n')
    await writeFile(join(dir, 'ADR-012-bar.md'), '# ADR-012: Bar\n\nbody\n')
    await writeFile(join(dir, 'README.md'), '# Ignored\n')
    const docs = await loadDecisions(dir)
    const ids = docs.map((d) => d.id).sort()
    expect(ids).toContain('conception')
    expect(ids).toContain('conception-baseline')
    expect(ids).toContain('adr-001-foo')
    expect(ids).toContain('adr-012-bar')
    // README.md must not be picked up.
    expect(ids).not.toContain('readme')
  })

  it('attaches kind and number to ADRs', async () => {
    const dir = await tempDir()
    await writeFile(join(dir, 'ADR-007-consent.md'), '# ADR-007: Consent\n')
    const [doc] = await loadDecisions(dir)
    expect(doc?.kind).toBe('adr')
    expect(doc?.number).toBe(7)
  })

  it('uses the first H1 as the title, with filename fallback', async () => {
    const dir = await tempDir()
    await writeFile(join(dir, 'ADR-002-no-heading.md'), 'just prose, no heading\n')
    const [doc] = await loadDecisions(dir)
    expect(doc?.title).toBe('ADR-002-no-heading')
  })
})

describe('loadChanges', () => {
  it('reads each <id>.yaml as a WorkItem keyed by filename', async () => {
    const dir = await tempDir()
    const id = 'wi-1-demo'
    await writeFile(
      join(dir, `${id}.yaml`),
      [
        'id: wi-1-demo',
        'title: Demo',
        'status: pending',
        'risk: low',
        'motivation: x',
        'scope: {}',
        'out_of_scope: []',
        'affected_guarantees: []',
        'verification: []',
        'risk_notes: x',
        'created_at: 2026-06-17',
        'authors: [tn]',
      ].join('\n') + '\n',
    )
    const changes = await loadChanges(dir)
    expect(changes.length).toBe(1)
    expect(changes[0]?.id).toBe(id)
    expect(changes[0]?.work_item.title).toBe('Demo')
    expect(changes[0]?.work_item.completion).toBeUndefined()
  })

  it('picks up completion when present in work-item.yaml', async () => {
    const dir = await tempDir()
    const id = 'wi-2-done'
    await writeFile(
      join(dir, `${id}.yaml`),
      'id: wi-2-done\ntitle: D\nstatus: completed\ncompletion:\n  summary: done\n',
    )
    const changes = await loadChanges(dir)
    expect(changes[0]?.work_item.completion?.summary).toBe('done')
  })

  it('skips files without YAML objects', async () => {
    const dir = await tempDir()
    await writeFile(join(dir, 'not-object.yaml'), '- item\n')
    const changes = await loadChanges(dir)
    expect(changes).toEqual([])
  })
})
