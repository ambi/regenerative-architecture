/**
 * Render CONCEPTION + ADR markdown documents into the Decisions tab.
 *
 * Pure: takes pre-loaded DecisionDoc records and emits HTML.
 */

import { badge, chip, esc } from './html.ts'
import { extractHeadings, firstParagraphText, renderMarkdown } from './markdown.ts'
import type { DecisionDoc } from './types.ts'

const renderDecisionCard = (doc: DecisionDoc): string => {
  const headings = extractHeadings(doc.body)
  const toc = headings.length
    ? `<details class="doc-toc"><summary>Outline (${headings.length})</summary><ol>${headings
        .map(
          (h) =>
            `<li class="lvl-${h.level}"><a href="#${esc(doc.id)}-${esc(h.anchor)}">${esc(h.text)}</a></li>`,
        )
        .join('')}</ol></details>`
    : ''
  const numberBadge =
    doc.kind === 'adr' && doc.number !== undefined
      ? badge(`ADR-${String(doc.number).padStart(3, '0')}`, 'adr-num')
      : doc.kind === 'conception'
        ? badge('CONCEPTION', 'conception')
        : ''
  return `<article class="card decision" id="${esc(doc.id)}">
    <header>
      <h3>${esc(doc.title)}</h3>
      ${numberBadge}
      ${chip(doc.filename, 'hint')}
    </header>
    ${toc}
    ${renderMarkdown(doc.body)}
  </article>`
}

export const renderDecisionsTab = (docs: DecisionDoc[]): string => {
  if (docs.length === 0)
    return renderEmptyTab('Decisions', 'No CONCEPTION or ADR sources were provided.')
  const conceptions = docs.filter((d) => d.kind === 'conception')
  const adrs = docs.filter((d) => d.kind === 'adr')
  const adrsSorted = [...adrs].sort((a, b) => (a.number ?? 999_999) - (b.number ?? 999_999))

  const conceptionSection = conceptions.length
    ? `<section id="dec-conception">
        <h2>Conception <span class="count">${conceptions.length}</span></h2>
        <p class="lead">期待する成果と必須事項、コンセプション・ベースライン。実装と SCL の起点になる文書。</p>
        <div class="cards">${conceptions.map((d) => renderDecisionCard(d)).join('')}</div>
      </section>`
    : ''

  const adrCards = adrsSorted.map((d) => renderDecisionCard(d)).join('')

  const adrIndex = adrsSorted
    .map((d) => {
      const num = d.number !== undefined ? `ADR-${String(d.number).padStart(3, '0')}` : '—'
      return `<a class="adr-index-row" href="#${esc(d.id)}">
        <code class="adr-num">${esc(num)}</code>
        <span class="adr-title">${esc(d.title)}</span>
        <span class="adr-preview">${firstParagraphText(d.body, 140)}</span>
      </a>`
    })
    .join('')
  const adrSection = adrs.length
    ? `<section id="dec-adrs">
        <h2>Architecture Decisions <span class="count">${adrs.length}</span></h2>
        <p class="lead">採用・棄却した設計判断とその根拠。SCL や CONCEPTION からトレースされる。</p>
        <div class="adr-index">${adrIndex}</div>
        <div class="cards">${adrCards}</div>
      </section>`
    : ''

  return `<section id="dec-overview" class="tab-overview">
    <header class="page-header">
      <div class="eyebrow">Decisions</div>
      <h1>CONCEPTION &amp; ADR</h1>
    </header>
    <div class="stats">
      <span class="stat"><span class="stat-num">${conceptions.length}</span><span class="stat-label">conception</span></span>
      <span class="stat"><span class="stat-num">${adrs.length}</span><span class="stat-label">adrs</span></span>
    </div>
  </section>
  ${conceptionSection}
  ${adrSection}`
}

export const decisionsTocItems = (docs: DecisionDoc[]): Array<{ id: string; label: string }> => {
  const items: Array<{ id: string; label: string }> = [{ id: 'dec-overview', label: 'Overview' }]
  if (docs.some((d) => d.kind === 'conception'))
    items.push({ id: 'dec-conception', label: 'Conception' })
  if (docs.some((d) => d.kind === 'adr')) items.push({ id: 'dec-adrs', label: 'ADRs' })
  return items
}

const renderEmptyTab = (label: string, message: string): string =>
  `<section id="${label.toLowerCase()}-empty" class="tab-overview">
    <header class="page-header">
      <div class="eyebrow">${esc(label)}</div>
      <h1>${esc(label)}</h1>
    </header>
    <p class="lead">${esc(message)}</p>
  </section>`
