/**
 * End-to-end render smoke tests — feed each renderer a tiny fixture and
 * assert key anchors / labels / cross-refs appear in the output.
 */

import { describe, expect, it } from 'bun:test'
import { renderPage } from './page.ts'
import { renderChangesTab } from './render-changes.ts'
import { renderDecisionsTab } from './render-decisions.ts'
import { renderSclTab, sclTocItems } from './render-scl.ts'
import type { ChangeEntry, DecisionDoc, SclDocument, SiteInput } from './types.ts'

const sampleScl = (): SclDocument => ({
  system: 'demo',
  spec_version: '1.0',
  vocabulary: {
    Foo: { definition: 'a thing' },
  },
  models: {
    Foo: { kind: 'entity', identity: ['a', 'b'], fields: { a: { type: 'String' } } },
    Bar: { kind: 'enum', values: ['X', 'Y'] },
    BarUpdated: { kind: 'event', payload: { id: { type: 'UUID' } } },
  },
  interfaces: {
    DoIt: {
      description: 'do it',
      steps: ['"{x}" を実行する'],
      input: { x: { type: 'String' } },
      output: { y: { type: 'Foo' } },
      emits: ['BarUpdated'],
      bindings: [
        { kind: 'http', method: 'POST', path: '/do' },
        { kind: 'schedule', every: '1m' },
      ],
    },
  },
  permissions: {
    P: { actor: 'User', action: 'Do', resource: 'Foo', allow_when: 'true' },
  },
  invariants: {
    I: { description: 'always', always: 'x == y' },
  },
  scenarios: {
    'demo の流れ': {
      steps: ['"DoIt" を呼ぶ', '"BarUpdated" が発行される'],
    },
  },
  objectives: {
    O: { kind: 'slo', metric: 'latency_p95', target: '<200ms' } as never,
  },
})

describe('renderSclTab', () => {
  const html = renderSclTab(sampleScl())

  it('contains every present-section anchor', () => {
    expect(html).toContain('id="vocabulary"')
    expect(html).toContain('id="models"')
    expect(html).toContain('id="interfaces"')
    expect(html).toContain('id="permissions"')
    expect(html).toContain('id="invariants"')
    expect(html).toContain('id="scenarios"')
    expect(html).toContain('id="objectives"')
  })

  it('renders the spec version in the SCL overview banner', () => {
    expect(html).toContain('spec 1.0')
  })

  it('renders composite identity as a comma-joined string', () => {
    expect(html).toContain('a, b')
  })

  it('linkifies known model names quoted inside scenario steps', () => {
    // "DoIt" should become a link to #iface-doit. "BarUpdated" → #model-barupdated.
    expect(html).toContain('href="#iface-doit"')
    expect(html).toContain('href="#model-barupdated"')
  })

  it('renders the http binding method as a badge', () => {
    expect(html).toContain('badge-method-post')
  })

  it('renders the schedule binding kind', () => {
    expect(html).toContain('every: 1m')
  })

  it('lists section titles in TOC items', () => {
    const items = sclTocItems(sampleScl())
    expect(items.map((i) => i.id)).toContain('models')
    expect(items.map((i) => i.id)).toContain('scenarios')
  })
})

describe('renderDecisionsTab', () => {
  const docs: DecisionDoc[] = [
    {
      id: 'conception',
      title: 'Conception',
      kind: 'conception',
      filename: 'CONCEPTION.md',
      body: '## Goals\n\nA paragraph.',
    },
    {
      id: 'adr-001-foo',
      title: 'ADR-001: Foo',
      kind: 'adr',
      filename: 'ADR-001-foo.md',
      body: '## Context\n\nWhy.',
      number: 1,
    },
  ]
  const html = renderDecisionsTab(docs)

  it('emits one card per document', () => {
    expect(html).toContain('id="conception"')
    expect(html).toContain('id="adr-001-foo"')
  })

  it('renders ADR number badges and an ADR index', () => {
    expect(html).toContain('ADR-001')
    expect(html).toContain('adr-index-row')
  })

  it('renders markdown bodies through the .md container', () => {
    expect(html).toContain('class="md"')
    expect(html).toContain('Goals')
  })

  it('shows an empty state for zero docs', () => {
    const empty = renderDecisionsTab([])
    expect(empty).toContain('No CONCEPTION or ADR sources')
  })
})

describe('renderChangesTab', () => {
  const changes: ChangeEntry[] = [
    {
      id: 'wi-1-demo',
      work_item: {
        id: 'wi-1-demo',
        title: 'Demo',
        status: 'in_progress',
        risk: 'medium',
        motivation: 'because',
        scope: { ui: ['screen A'] },
        out_of_scope: ['unrelated'],
        affected_guarantees: ['rbac'],
        verification: [{ cmd: 'go test ./...' }],
        risk_notes: 'careful',
        target_state: {
          scl: ['new interface'],
          ui: ['screen B'],
        },
      },
    },
    {
      id: 'wi-2-done',
      work_item: { id: 'wi-2-done', title: 'Done thing', status: 'completed', risk: 'low' },
      completion_report: {
        id: 'wi-2-done',
        title: 'Done thing',
        status: 'completed',
        summary: 'finished',
        affected_guarantees_state: ['unchanged'],
        scl_changes: {
          models: ['Foo'],
        },
      },
    },
  ]
  const html = renderChangesTab(changes)

  it('renders one card per change anchored by id', () => {
    expect(html).toContain('id="wi-1-demo"')
    expect(html).toContain('id="wi-2-done"')
  })

  it('shows status badges and an index row per change', () => {
    expect(html).toContain('badge-status-progress')
    expect(html).toContain('badge-status-done')
    expect(html).toContain('ch-index-row')
  })

  it('renders the completion report under the work item when present', () => {
    expect(html).toContain('Completion report')
    expect(html).toContain('finished')
  })

  it('renders schema-extension fields instead of dropping them', () => {
    expect(html).toContain('Target State')
    expect(html).toContain('new interface')
    expect(html).toContain('Scl Changes')
    expect(html).toContain('Foo')
  })

  it('orders in_progress before completed in the details list', () => {
    const first = html.indexOf('id="wi-1-demo"')
    const second = html.indexOf('id="wi-2-done"')
    expect(first).toBeGreaterThan(-1)
    expect(second).toBeGreaterThan(first)
  })

  it('emits an empty-state placeholder for zero changes', () => {
    expect(renderChangesTab([])).toContain('No work items')
  })
})

describe('renderPage (integration)', () => {
  const site: SiteInput = {
    scl: sampleScl(),
    decisions: [
      {
        id: 'adr-001-foo',
        title: 'Foo',
        kind: 'adr',
        filename: 'ADR-001-foo.md',
        body: 'body',
        number: 1,
      },
    ],
    changes: [
      {
        id: 'wi-x',
        work_item: { id: 'wi-x', title: 'X', status: 'pending', risk: 'low' },
      },
    ],
    title: 'demo system',
  }
  const html = renderPage(site)

  it('produces a single self-contained HTML document', () => {
    expect(html.startsWith('<!doctype html>')).toBe(true)
    expect(html).toContain('<style>')
    expect(html).toContain('<script>')
  })

  it('embeds all four tabs with data-tab markers', () => {
    expect(html).toContain('data-tab="overview"')
    expect(html).toContain('data-tab="scl"')
    expect(html).toContain('data-tab="decisions"')
    expect(html).toContain('data-tab="changes"')
  })

  it('emits a tab bar with active overview by default (server side)', () => {
    expect(html).toContain('tab-link active')
    expect(html).toContain('data-tab-link="overview"')
  })

  it('honours --title override in <title> and header', () => {
    expect(html).toContain('<title>demo system</title>')
    expect(html).toContain('>demo system<')
  })

  it('renders without injected XSS when SCL strings contain script tags', () => {
    const evil: SiteInput = {
      ...site,
      scl: { system: '<script>alert(1)</script>', spec_version: '1.0' },
    }
    const out = renderPage(evil)
    expect(out).not.toContain('<script>alert(1)</script>')
    expect(out).toContain('&lt;script&gt;alert(1)&lt;/script&gt;')
  })

  it('is deterministic on identical input', () => {
    expect(renderPage(site)).toBe(renderPage(site))
  })
})
