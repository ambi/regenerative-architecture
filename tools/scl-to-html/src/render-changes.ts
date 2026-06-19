/**
 * Render work items and their optional completion records.
 *
 * Each change directory becomes one card with header badges (status,
 * risk, dates) and three collapsible blocks: motivation, scope, and
 * verification. When completion is present its summary
 * and residual risk hang underneath.
 */

import { badge, chip, esc, isObj, kvRow, renderValue, slug } from './html.ts'
import { renderMarkdown } from './markdown.ts'
import type { ChangeEntry, Completion, WorkItem } from './types.ts'

const STATUS_KIND: Record<string, string> = {
  pending: 'status-pending',
  in_progress: 'status-progress',
  completed: 'status-done',
  cancelled: 'status-cancelled',
}

const RISK_KIND: Record<string, string> = {
  low: 'risk-low',
  medium: 'risk-medium',
  high: 'risk-high',
  critical: 'risk-critical',
}

const renderProse = (text: string | undefined): string => {
  if (!text) return ''
  return renderMarkdown(text)
}

const titleize = (key: string): string =>
  key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())

const renderChangeValue = (v: unknown): string => {
  if (v === undefined || v === null) return '<span class="muted">—</span>'
  if (typeof v === 'string') return renderProse(v)
  if (typeof v === 'number' || typeof v === 'boolean') return `<code>${esc(v)}</code>`
  if (Array.isArray(v)) {
    if (v.length === 0) return '<p class="muted">(empty)</p>'
    return `<ul class="change-list">${v
      .map((item) => `<li>${renderInlineItem(item)}</li>`)
      .join('')}</ul>`
  }
  if (isObj(v)) {
    const rows = Object.entries(v)
      .map(([k, val]) => kvRow(k, renderChangeValue(val)))
      .join('')
    return `<dl class="kv change-kv">${rows}</dl>`
  }
  return renderValue(v)
}

const renderListOrText = (v: unknown): string => {
  if (v === undefined || v === null) return ''
  return renderChangeValue(v)
}

const renderInlineItem = (item: unknown): string => {
  if (typeof item === 'string') return renderProse(item)
  if (isObj(item)) {
    const obj = item
    if (typeof obj.cmd === 'string') {
      const where = typeof obj.in === 'string' ? ` ${chip(`in: ${obj.in}`, 'hint')}` : ''
      const reason =
        typeof obj.reason === 'string' ? ` <span class="muted">— ${esc(obj.reason)}</span>` : ''
      const result =
        obj.result !== undefined
          ? ` <span class="muted">→ ${esc(typeof obj.result === 'string' ? obj.result : JSON.stringify(obj.result))}</span>`
          : ''
      const rest = Object.fromEntries(
        Object.entries(obj).filter(([k]) => !['cmd', 'in', 'reason', 'result'].includes(k)),
      )
      const extra = Object.keys(rest).length ? renderChangeValue(rest) : ''
      return `<code>${esc(obj.cmd)}</code>${where}${reason}${result}${extra}`
    }
    return renderChangeValue(item)
  }
  return renderChangeValue(item)
}

const renderScope = (scope: unknown): string => {
  if (scope === undefined || scope === null) return ''
  if (typeof scope === 'string') return renderProse(scope)
  if (Array.isArray(scope)) return renderListOrText(scope)
  if (typeof scope === 'object') {
    const entries = Object.entries(scope as Record<string, unknown>)
    if (entries.length === 0) return '<p class="muted">(empty)</p>'
    return entries
      .map(([k, v]) => `<div class="scope-group"><h4>${esc(k)}</h4>${renderListOrText(v)}</div>`)
      .join('')
  }
  return renderValue(scope)
}

const renderExtraBlocks = (
  source: Record<string, unknown>,
  knownKeys: readonly string[],
): string[] => {
  const known = new Set(knownKeys)
  return Object.entries(source)
    .filter(([key, value]) => !known.has(key) && value !== undefined)
    .map(
      ([key, value]) =>
        `<details class="change-block"><summary>${esc(titleize(key))}</summary>${renderChangeValue(value)}</details>`,
    )
}

const WORK_ITEM_KEYS = [
  'id',
  'title',
  'status',
  'created_at',
  'authors',
  'risk',
  'motivation',
  'scope',
  'out_of_scope',
  'affected_guarantees',
  'verification',
  'risk_notes',
  'completion',
] as const

const COMPLETION_KEYS = [
  'completed_at',
  'summary',
  'semantic_diff',
  'verification',
  'affected_guarantees_state',
  'remaining_guarantees_state',
  'residual_risk',
  'traceability',
  'human_decisions',
  'approver_note',
] as const

const renderWorkItem = (wi: WorkItem): string => {
  const statusKind = STATUS_KIND[wi.status ?? ''] ?? 'status-pending'
  const riskKind = wi.risk ? (RISK_KIND[wi.risk] ?? '') : ''
  const meta = [
    wi.status ? badge(wi.status, statusKind) : '',
    wi.risk ? badge(`risk: ${wi.risk}`, riskKind) : '',
    wi.created_at ? chip(`created ${wi.created_at}`, 'hint') : '',
    wi.authors?.length ? chip(`authors: ${wi.authors.join(', ')}`, 'hint') : '',
  ]
    .filter(Boolean)
    .join(' ')
  const blocks: string[] = []
  if (wi.motivation) {
    blocks.push(
      `<details class="change-block" open><summary>Motivation</summary>${renderProse(wi.motivation)}</details>`,
    )
  }
  if (wi.scope !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Scope</summary>${renderScope(wi.scope)}</details>`,
    )
  }
  if (wi.out_of_scope !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Out of scope</summary>${renderListOrText(wi.out_of_scope)}</details>`,
    )
  }
  if (wi.affected_guarantees !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Affected guarantees</summary>${renderListOrText(wi.affected_guarantees)}</details>`,
    )
  }
  if (wi.verification !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Verification</summary>${renderListOrText(wi.verification)}</details>`,
    )
  }
  if (wi.risk_notes) {
    blocks.push(
      `<details class="change-block"><summary>Risk notes</summary>${renderProse(wi.risk_notes)}</details>`,
    )
  }
  blocks.push(...renderExtraBlocks(wi, WORK_ITEM_KEYS))
  return `<div class="work-item">
    <header class="wi-header">
      <h4>${esc(wi.title ?? wi.id)}</h4>
      <div class="wi-meta">${meta}</div>
    </header>
    ${blocks.join('')}
  </div>`
}

const renderCompletion = (completion: Completion, status: string | undefined): string => {
  const meta = [
    status ? badge(status, STATUS_KIND[status] ?? '') : '',
    completion.completed_at ? chip(`completed ${completion.completed_at}`, 'hint') : '',
  ]
    .filter(Boolean)
    .join(' ')
  const blocks: string[] = []
  if (completion.summary) {
    blocks.push(
      `<details class="change-block" open><summary>Summary</summary>${renderProse(completion.summary)}</details>`,
    )
  }
  if (completion.semantic_diff !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Semantic diff</summary>${renderListOrText(completion.semantic_diff)}</details>`,
    )
  }
  if (completion.verification !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Verification results</summary>${renderListOrText(completion.verification)}</details>`,
    )
  }
  const guarantees = completion.affected_guarantees_state ?? completion.remaining_guarantees_state
  if (guarantees !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Guarantees state</summary>${renderListOrText(guarantees)}</details>`,
    )
  }
  if (completion.residual_risk !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Residual risk</summary>${renderListOrText(completion.residual_risk)}</details>`,
    )
  }
  if (completion.traceability !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Traceability</summary>${renderListOrText(completion.traceability)}</details>`,
    )
  }
  if (completion.human_decisions !== undefined) {
    blocks.push(
      `<details class="change-block"><summary>Human decisions</summary>${renderListOrText(completion.human_decisions)}</details>`,
    )
  }
  if (completion.approver_note) {
    blocks.push(
      `<details class="change-block"><summary>Approver note</summary>${renderProse(completion.approver_note)}</details>`,
    )
  }
  blocks.push(...renderExtraBlocks(completion, COMPLETION_KEYS))
  return `<div class="completion-record">
    <header class="cr-header">
      <h4>Completion</h4>
      <div class="cr-meta">${meta}</div>
    </header>
    ${blocks.join('')}
  </div>`
}

const renderChangeCard = (entry: ChangeEntry): string => {
  const wi = entry.work_item
  return `<article class="card change" id="${esc(slug(entry.id))}">
    <header>
      <h3>${esc(wi.title ?? entry.id)}</h3>
      ${chip(entry.id, 'hint')}
      ${wi.completion ? badge('has completion', 'has-cr') : ''}
    </header>
    ${renderWorkItem(wi)}
    ${wi.completion ? renderCompletion(wi.completion, wi.status) : ''}
  </article>`
}

export const renderChangesTab = (changes: ChangeEntry[]): string => {
  if (changes.length === 0) {
    return `<section id="ch-empty" class="tab-overview">
      <header class="page-header">
        <div class="eyebrow">Work Items</div>
        <h1>Work Items</h1>
      </header>
      <p class="lead">No work items were provided.</p>
    </section>`
  }

  // Stats by status
  const byStatus = new Map<string, number>()
  for (const c of changes) {
    const s = c.work_item.status ?? 'pending'
    byStatus.set(s, (byStatus.get(s) ?? 0) + 1)
  }
  const stats = ['pending', 'in_progress', 'completed', 'cancelled']
    .filter((s) => byStatus.has(s))
    .map(
      (s) =>
        `<span class="stat"><span class="stat-num">${byStatus.get(s) ?? 0}</span><span class="stat-label">${esc(s.replace(/_/g, ' '))}</span></span>`,
    )
    .join('')

  // Sorted: in_progress → pending → completed → cancelled, then by id descending.
  const statusOrder: Record<string, number> = {
    in_progress: 0,
    pending: 1,
    completed: 2,
    cancelled: 3,
  }
  const sorted = [...changes].sort((a, b) => {
    const sa = statusOrder[a.work_item.status ?? 'pending'] ?? 1
    const sb = statusOrder[b.work_item.status ?? 'pending'] ?? 1
    if (sa !== sb) return sa - sb
    return b.id.localeCompare(a.id)
  })

  const index = sorted
    .map((c) => {
      const wi = c.work_item
      return `<a class="ch-index-row" href="#${esc(slug(c.id))}">
        <code class="ch-id">${esc(c.id)}</code>
        <span class="ch-title">${esc(wi.title ?? c.id)}</span>
        ${wi.status ? badge(wi.status, STATUS_KIND[wi.status] ?? '') : ''}
        ${wi.risk ? badge(wi.risk, RISK_KIND[wi.risk] ?? '') : ''}
        ${wi.completion ? chip('completion', 'has-cr') : ''}
      </a>`
    })
    .join('')

  const cards = sorted.map((c) => renderChangeCard(c)).join('')

  return `<section id="ch-overview" class="tab-overview">
    <header class="page-header">
      <div class="eyebrow">Work Items</div>
      <h1>Work items</h1>
    </header>
    <div class="stats">${stats}</div>
  </section>
  <section id="ch-index">
    <h2>Index</h2>
    <p class="lead">優先度順: in_progress → pending → completed → cancelled。同区分内は新しい id 順。</p>
    <div class="ch-index">${index}</div>
  </section>
  <section id="ch-details">
    <h2>Details <span class="count">${sorted.length}</span></h2>
    <div class="cards">${cards}</div>
  </section>`
}

export const changesTocItems = (changes: ChangeEntry[]): Array<{ id: string; label: string }> => {
  if (changes.length === 0) return [{ id: 'ch-empty', label: 'Overview' }]
  return [
    { id: 'ch-overview', label: 'Overview' },
    { id: 'ch-index', label: 'Index' },
    { id: 'ch-details', label: 'Details' },
  ]
}
