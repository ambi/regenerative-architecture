/**
 * Top-level HTML shell — tabs, sidebar TOC, CSS, scroll-spy / hash-router JS.
 *
 * Layout:
 *   header  · system title + tab bar
 *   main    · sidebar (per-tab TOC) + the four <div class="tab"> panes
 *
 * All four panes are rendered into the same HTML; client-side JS picks
 * one based on `#tab=<name>` (with the section anchor as a sibling
 * `&sec=<id>`). Without JS the page degrades to a single scrollable
 * document (every pane visible).
 */

import { esc } from './html.ts'
import { renderChangesTab, changesTocItems } from './render-changes.ts'
import { renderDecisionsTab, decisionsTocItems } from './render-decisions.ts'
import { renderSclTab, sclTocItems } from './render-scl.ts'
import type { SiteInput } from './types.ts'

type TabKey = 'overview' | 'scl' | 'decisions' | 'work-items'

const TAB_LABELS: Record<TabKey, string> = {
  overview: 'Overview',
  scl: 'SCL',
  decisions: 'Decisions',
  'work-items': 'Work Items',
}

const renderOverviewTab = (site: SiteInput): string => {
  const { scl, decisions, work_items: workItems } = site
  const sclSectionCount = sclTocItems(scl).length - 1
  const adrCount = decisions.filter((d) => d.kind === 'adr').length
  const conceptionCount = decisions.filter((d) => d.kind === 'conception').length
  const inProgress = workItems.filter((c) => c.work_item.status === 'in_progress').length
  const pending = workItems.filter((c) => c.work_item.status === 'pending').length
  const completed = workItems.filter((c) => c.work_item.status === 'completed').length

  return `<section id="ov-hero" class="tab-overview">
    <header class="page-header">
      <div class="eyebrow">Regenerative Architecture</div>
      <h1>${esc(site.title ?? scl.system)}</h1>
      <p class="lead">SCL spec ${esc(scl.spec_version)} — 仕様核と、それを支える設計判断・ワークアイテムを一つの文書にまとめたもの。</p>
    </header>
    <div class="overview-grid">
      <a class="overview-tile" href="#tab=scl">
        <div class="overview-tile-label">SCL</div>
        <div class="overview-tile-num">${sclSectionCount}</div>
        <div class="overview-tile-hint">sections present</div>
      </a>
      <a class="overview-tile" href="#tab=decisions">
        <div class="overview-tile-label">Decisions</div>
        <div class="overview-tile-num">${adrCount}</div>
        <div class="overview-tile-hint">ADRs · ${conceptionCount} conception doc${conceptionCount === 1 ? '' : 's'}</div>
      </a>
      <a class="overview-tile" href="#tab=work-items">
        <div class="overview-tile-label">Work Items</div>
        <div class="overview-tile-num">${workItems.length}</div>
        <div class="overview-tile-hint">${inProgress} in progress · ${pending} pending · ${completed} done</div>
      </a>
    </div>
  </section>`
}

const renderTab = (key: TabKey, body: string): string =>
  `<div class="tab" data-tab="${esc(key)}" id="tab-${esc(key)}">${body}</div>`

const renderTabBar = (active: TabKey): string => {
  const tabs = (['overview', 'scl', 'decisions', 'work-items'] as TabKey[])
    .map(
      (key) =>
        `<a class="tab-link${key === active ? ' active' : ''}" data-tab-link="${esc(key)}" href="#tab=${esc(key)}">${esc(TAB_LABELS[key])}</a>`,
    )
    .join('')
  return `<nav class="tab-bar" aria-label="Tabs">${tabs}</nav>`
}

const renderTocFor = (key: TabKey, items: Array<{ id: string; label: string }>): string => {
  const list = items
    .map(
      (item) =>
        `<li><a data-sec="${esc(item.id)}" href="#tab=${esc(key)}&sec=${esc(item.id)}">${esc(item.label)}</a></li>`,
    )
    .join('')
  return `<nav class="toc" data-toc-for="${esc(key)}" aria-label="${esc(TAB_LABELS[key])} contents">
    <div class="toc-title">${esc(TAB_LABELS[key])}</div>
    <ol>${list}</ol>
  </nav>`
}

const CSS = `
:root { color-scheme: light dark;
  --bg:#fbfbfd; --surface:#ffffff; --surface-2:#f4f4f7;
  --fg:#1c1c1f; --fg-soft:#54545a; --muted:#8a8a92;
  --border:#e5e5ea; --accent:#5b5be6; --accent-soft:#eeeefb;
  --ok:#117a3d; --ok-soft:rgba(17,122,61,.12);
  --warn:#a25400; --warn-soft:rgba(162,84,0,.14);
  --danger:#b3261e; --danger-soft:rgba(179,38,30,.1);
  --code-bg:#f1f1f5; --shadow:0 1px 2px rgba(0,0,0,.04),0 4px 16px rgba(0,0,0,.04);
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg:#0f1115; --surface:#16181f; --surface-2:#1c1f27;
    --fg:#e6e6ea; --fg-soft:#b0b0b8; --muted:#7a7a85;
    --border:#2a2d36; --accent:#8b8bff; --accent-soft:#1f2030;
    --ok:#5bcc8d; --ok-soft:rgba(91,204,141,.15);
    --warn:#e0a04f; --warn-soft:rgba(224,160,79,.15);
    --danger:#ff7a72; --danger-soft:rgba(255,122,114,.15);
    --code-bg:#1c1f27; --shadow:0 1px 2px rgba(0,0,0,.4),0 4px 16px rgba(0,0,0,.3);
  }
}
*, *::before, *::after { box-sizing: border-box; }
html, body { margin: 0; padding: 0; }
body {
  background: var(--bg); color: var(--fg);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Hiragino Sans', 'Noto Sans JP', system-ui, sans-serif;
  font-size: 15px; line-height: 1.6; -webkit-font-smoothing: antialiased;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
code, pre, .name, .path, .type, .chip, .expr {
  font-family: ui-monospace, 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
}
code { background: var(--code-bg); padding: 1px 6px; border-radius: 4px; font-size: 0.88em; }

/* top header */
.app-header {
  position: sticky; top: 0; z-index: 10;
  background: color-mix(in srgb, var(--bg) 90%, transparent);
  backdrop-filter: saturate(180%) blur(8px); border-bottom: 1px solid var(--border);
}
.app-header-row {
  display: flex; gap: 24px; align-items: center;
  max-width: 1320px; margin: 0 auto; padding: 12px 24px;
}
.app-title {
  font-weight: 700; letter-spacing: -0.01em; font-size: 15px; color: var(--fg);
}
.tab-bar { display: flex; gap: 4px; }
.tab-link {
  padding: 6px 12px; border-radius: 8px; font-size: 14px; color: var(--fg-soft);
  border: 1px solid transparent;
}
.tab-link:hover { background: var(--surface-2); text-decoration: none; }
.tab-link.active {
  background: var(--accent-soft); color: var(--accent); font-weight: 600;
  border-color: color-mix(in srgb, var(--accent) 30%, transparent);
}

/* layout */
.layout {
  display: grid; grid-template-columns: 240px minmax(0,1fr); gap: 40px;
  max-width: 1320px; margin: 0 auto; padding: 24px 24px 96px;
}
@media (max-width: 900px) {
  .layout { grid-template-columns: 1fr; padding: 16px; gap: 24px; }
  .toc { position: static; max-height: none; }
}
.toc {
  position: sticky; top: 80px; align-self: start;
  max-height: calc(100vh - 100px); overflow-y: auto; font-size: 14px;
  display: none;
}
.toc.active { display: block; }
.toc-title {
  font-size: 11px; font-weight: 700; letter-spacing: 0.12em; text-transform: uppercase;
  color: var(--muted); margin-bottom: 12px;
}
.toc ol { list-style: none; padding: 0; margin: 0; }
.toc li { padding: 4px 0; }
.toc a {
  display: block; padding: 4px 8px 4px 10px; border-radius: 6px;
  color: var(--fg-soft); border-left: 2px solid transparent;
  transition: background-color .12s, color .12s, border-color .12s;
}
.toc a:hover { background: var(--surface-2); color: var(--fg); text-decoration: none; }
.toc a.active {
  background: var(--accent-soft); color: var(--accent); font-weight: 600;
  border-left-color: var(--accent);
}
main { min-width: 0; }

/* tab panes — JS hides inactive ones; no-JS shows all */
.tab { display: block; }
.js .tab { display: none; }
.js .tab.active { display: block; }

section { margin-bottom: 56px; scroll-margin-top: 88px; }

/* overview tab */
.tab-overview { margin-bottom: 32px; }
.page-header { margin-bottom: 24px; }
.eyebrow {
  font-size: 11px; font-weight: 700; letter-spacing: 0.18em; text-transform: uppercase;
  color: var(--accent); margin-bottom: 4px;
}
.page-header h1 { font-size: 38px; letter-spacing: -0.02em; margin: 0 0 8px; }
.page-meta { display: flex; gap: 8px; flex-wrap: wrap; }
.stats {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px; margin-top: 8px;
}
.stat {
  background: var(--surface); border: 1px solid var(--border); border-radius: 10px;
  padding: 14px; color: var(--fg); display: flex; flex-direction: column; gap: 2px;
  transition: border-color .15s, transform .15s;
}
.stat:hover { border-color: var(--accent); text-decoration: none; transform: translateY(-1px); }
.stat-num { font-size: 24px; font-weight: 700; letter-spacing: -0.02em; }
.stat-label { font-size: 12px; color: var(--muted); text-transform: capitalize; }

.overview-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px; margin-top: 24px;
}
.overview-tile {
  background: var(--surface); border: 1px solid var(--border); border-radius: 14px;
  padding: 22px; color: var(--fg); display: flex; flex-direction: column; gap: 6px;
  transition: border-color .15s, transform .15s;
}
.overview-tile:hover {
  border-color: var(--accent); text-decoration: none; transform: translateY(-1px);
}
.overview-tile-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.14em; text-transform: uppercase;
  color: var(--accent);
}
.overview-tile-num { font-size: 36px; font-weight: 700; letter-spacing: -0.02em; }
.overview-tile-hint { font-size: 13px; color: var(--muted); }

section > h2 {
  font-size: 26px; margin: 0 0 6px; letter-spacing: -0.01em;
  border-bottom: 1px solid var(--border); padding-bottom: 10px;
  display: flex; align-items: baseline; gap: 10px;
}
section > h2 .count, .grp-title .count {
  font-size: 13px; font-weight: 500; color: var(--muted);
  background: var(--surface-2); padding: 2px 8px; border-radius: 999px;
  border: 1px solid var(--border);
}
.lead { color: var(--fg-soft); margin: 8px 0 20px; }

.group { margin-top: 24px; }
.grp-title {
  font-size: 14px; font-weight: 600; color: var(--fg-soft); text-transform: capitalize;
  margin: 16px 0 12px; display: flex; align-items: baseline; gap: 8px;
}
.grp-title .count { font-size: 11px; padding: 1px 7px; }

/* card */
.cards { display: grid; gap: 14px; }
.card {
  background: var(--surface); border: 1px solid var(--border); border-radius: 12px;
  padding: 18px 20px; box-shadow: var(--shadow);
  transition: border-color .15s, box-shadow .15s;
}
.card:target {
  border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft), var(--shadow);
}
.card header {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 8px;
}
.card header h3 { font-size: 17px; font-weight: 600; letter-spacing: -0.01em; margin: 0; }
.desc { color: var(--fg-soft); margin: 4px 0 12px; white-space: pre-wrap; }
.label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); margin: 10px 0 4px;
}
.label.ok { color: var(--ok); }
.label.danger { color: var(--danger); }

/* badges */
.badge {
  display: inline-block; font-size: 11px; font-weight: 600;
  padding: 2px 8px; border-radius: 999px;
  background: var(--accent-soft); color: var(--accent);
  letter-spacing: 0.02em; border: 1px solid transparent;
}
.badge-required { background: var(--danger-soft); color: var(--danger); }
.badge-optional { background: var(--surface-2); color: var(--muted); border-color: var(--border); }
.badge-readonly, .badge-idempotent {
  background: var(--surface-2); color: var(--fg-soft); border-color: var(--border);
}
.badge-version { background: var(--surface-2); color: var(--fg-soft); border-color: var(--border); }
.badge-context { background: var(--warn-soft); color: var(--warn); }
.badge-kind-entity { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-kind-value_object { background: var(--surface-2); color: var(--fg-soft); border: 1px solid var(--border); }
.badge-kind-enum { background: var(--ok-soft); color: var(--ok); }
.badge-kind-event { background: var(--warn-soft); color: var(--warn); }
.badge-kind-error { background: var(--danger-soft); color: var(--danger); }
.badge-kind-slo, .badge-kind-lifetime, .badge-kind-security, .badge-kind-retention {
  background: var(--accent-soft); color: var(--accent);
}
.badge-method-get { background: var(--ok-soft); color: var(--ok); }
.badge-method-post { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-method-put, .badge-method-patch { background: var(--warn-soft); color: var(--warn); }
.badge-method-delete { background: var(--danger-soft); color: var(--danger); }
.badge-severity-must, .badge-severity-critical { background: var(--danger-soft); color: var(--danger); }
.badge-severity-should, .badge-severity-high { background: var(--warn-soft); color: var(--warn); }
.badge-severity-medium { background: var(--warn-soft); color: var(--warn); }
.badge-severity-low { background: var(--ok-soft); color: var(--ok); }
.badge-adoption-required { background: var(--ok-soft); color: var(--ok); }
.badge-adoption-optional { background: var(--warn-soft); color: var(--warn); }
.badge-adoption-excluded { background: var(--danger-soft); color: var(--danger); }
.badge-strength, .badge-category {
  background: var(--surface-2); color: var(--fg-soft); border-color: var(--border);
}
.badge-status-pending { background: var(--surface-2); color: var(--fg-soft); border-color: var(--border); }
.badge-status-progress { background: var(--accent-soft); color: var(--accent); }
.badge-status-done { background: var(--ok-soft); color: var(--ok); }
.badge-status-cancelled { background: var(--surface-2); color: var(--muted); border-color: var(--border); }
.badge-risk-low { background: var(--ok-soft); color: var(--ok); }
.badge-risk-medium { background: var(--warn-soft); color: var(--warn); }
.badge-risk-high { background: var(--danger-soft); color: var(--danger); }
.badge-risk-critical { background: var(--danger-soft); color: var(--danger); font-weight: 700; }
.badge-has-cr { background: var(--ok-soft); color: var(--ok); }
.badge-adr-num { background: var(--accent-soft); color: var(--accent); font-weight: 700; }
.badge-conception { background: var(--warn-soft); color: var(--warn); font-weight: 700; }

/* chips */
.chip {
  display: inline-block; background: var(--surface-2); color: var(--fg-soft);
  border: 1px solid var(--border); padding: 1px 8px; border-radius: 6px;
  margin: 2px 2px 2px 0; font-size: 12px;
}
a.chip { transition: border-color .15s, color .15s; }
a.chip:hover { color: var(--accent); border-color: var(--accent); text-decoration: none; }
.chip-alias { background: var(--surface-2); }
.chip-constraint { background: var(--accent-soft); color: var(--accent); border-color: transparent; }
.chip-annotation { background: var(--warn-soft); color: var(--warn); border-color: transparent; }
.chip-default { background: var(--surface-2); color: var(--muted); }
.chip-enum { background: var(--ok-soft); color: var(--ok); border-color: transparent; }
.chip-event-ref { background: var(--warn-soft); color: var(--warn); border-color: transparent; }
.chip-error-ref { background: var(--danger-soft); color: var(--danger); border-color: transparent; }
.chip-iface-ref { background: rgba(91,91,230,.14); color: var(--accent); border-color: transparent; }
.chip-initial { background: rgba(91,91,230,.14); color: var(--accent); border-color: transparent; font-weight: 600; }
.chip-terminal { background: var(--warn-soft); color: var(--warn); border-color: transparent; }
.chip-tag { background: var(--surface-2); color: var(--fg-soft); }
.chip-hint { background: var(--surface-2); color: var(--muted); }
.chip-has-cr { background: var(--ok-soft); color: var(--ok); border-color: transparent; }

.name { color: var(--fg); font-weight: 600; }
.type { color: var(--fg-soft); }
.path { background: var(--surface-2); border: 1px solid var(--border); padding: 2px 8px; border-radius: 6px; font-size: 13px; }
.muted { color: var(--muted); }
.text { color: var(--fg); }

/* kv */
.kv {
  display: grid; grid-template-columns: minmax(120px, max-content) 1fr;
  gap: 4px 14px; align-items: baseline; margin: 6px 0;
}
.kv dt { color: var(--muted); font-size: 13px; font-weight: 500; padding-top: 2px; }
.kv dd { margin: 0; min-width: 0; color: var(--fg); }
.kv dl.kv { margin: 0; }
.vlist { margin: 4px 0 0; padding-left: 18px; }
.vlist li { margin: 3px 0; }
.chip-row { display: flex; flex-wrap: wrap; gap: 4px; }

/* tables */
table.fields { width: 100%; border-collapse: collapse; font-size: 14px; margin: 8px 0; }
table.fields th, table.fields td {
  text-align: left; padding: 8px 10px;
  border-bottom: 1px solid var(--border); vertical-align: top;
}
table.fields thead th {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); border-bottom: 2px solid var(--border);
}
table.fields tbody tr:last-child td { border-bottom: none; }
.enum-vals { display: flex; flex-wrap: wrap; gap: 4px; }

/* vocabulary */
.vocab-list { display: grid; gap: 12px; margin-top: 12px; }
.vocab-entry {
  background: var(--surface); border: 1px solid var(--border); border-radius: 12px;
  padding: 14px 18px;
  display: grid; grid-template-columns: minmax(160px, 200px) 1fr; gap: 18px;
  align-items: start;
}
.vocab-entry:target { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
@media (max-width: 720px) { .vocab-entry { grid-template-columns: 1fr; } }
.vocab-name { padding-top: 2px; display: flex; flex-wrap: wrap; gap: 6px; align-items: baseline; }
.vocab-name .name { font-size: 15px; }
.def { margin: 0 0 8px; color: var(--fg); }
.aliases { font-size: 13px; color: var(--fg-soft); }
.aliases-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); margin-right: 6px;
}
.ntcw {
  margin-top: 10px; padding: 8px 10px;
  background: var(--warn-soft); border-radius: 8px; font-size: 13px;
}
.ntcw-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--warn); margin-bottom: 4px;
}
.ntcw ul { margin: 0; padding-left: 18px; color: var(--fg-soft); }
.ntcw li { margin: 2px 0; }

/* state machine */
.sm-row { display: flex; gap: 12px; flex-wrap: wrap; align-items: baseline; margin: 6px 0 8px; }
.sm-row .label { min-width: 100px; margin: 0; }
.sub { margin-top: 10px; }
.io { margin-top: 10px; }

/* scenarios */
.iface-step { margin: 6px 0 2px; }
.step-tpl { color: var(--fg-soft); }
.scn-steps { margin: 8px 0 0; padding-left: 26px; }
.scn-step { padding: 3px 0; line-height: 1.7; }
.scn-step::marker { color: var(--muted); font-size: 0.85em; }
.step-arg { background: var(--surface-2); color: var(--fg); border: 1px solid var(--border); padding: 0 5px; border-radius: 4px; }
.chip-ref { background: var(--accent-soft); color: var(--accent); border-color: transparent; }

/* standards / requirements */
.requirements { display: grid; gap: 10px; margin-top: 12px; }
.requirement { border: 1px solid var(--border); border-radius: 9px; padding: 12px 14px; background: var(--surface-2); }
.requirement:target { border-color: var(--accent); }
.requirement header { display: flex; flex-wrap: wrap; align-items: center; gap: 7px; }
.requirement p { margin: 7px 0 0; }
.exclusion-reason { color: var(--danger); }
.reference-row { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin-top: 8px; }
.reference-label {
  min-width: 92px; color: var(--muted); font-size: 11px; font-weight: 700;
  letter-spacing: .08em; text-transform: uppercase;
}

/* clauses & expressions */
.clause { margin: 10px 0; padding: 10px 12px; background: var(--surface-2); border-radius: 8px; }
.clause-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); margin-bottom: 6px;
}
.clause-label.ok { color: var(--ok); }
.clause-label.danger { color: var(--danger); }
.clause-label.accent { color: var(--accent); }
.clause-label.muted { color: var(--muted); }
.expr { display: inline-block; padding: 2px 8px; background: var(--surface); border: 1px solid var(--border); border-radius: 4px; font-size: 13px; }
.expr-list { margin: 0; padding-left: 18px; }
.expr-list li { margin: 4px 0; }
.expr-op { margin: 6px 0 6px 12px; }
.expr-op-label { font-size: 11px; font-weight: 700; letter-spacing: 0.08em; color: var(--accent); margin-bottom: 4px; }
.expr-quant {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
  margin: 4px 0 4px 12px; padding: 4px 8px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.expr-quant-sym { color: var(--accent); font-weight: 700; font-size: 14px; }
.clause-assuming { background: var(--surface); border: 1px solid var(--border); }
.clause-eventually { background: var(--accent-soft); }
.within { font-size: 11px; font-weight: 500; color: var(--muted); text-transform: none; letter-spacing: 0; margin-left: 6px; }
.acceptance { margin: 6px 0; padding: 8px 10px; background: var(--surface); border: 1px solid var(--border); border-radius: 6px; }
.acceptance-label { font-size: 11px; font-weight: 700; letter-spacing: 0.08em; color: var(--accent); margin-bottom: 4px; }
.acceptance-leaf { margin: 4px 0; }

/* bindings */
.bindings { display: grid; gap: 10px; margin-top: 6px; }
.binding {
  display: grid; grid-template-columns: minmax(80px, max-content) 1fr; gap: 12px;
  align-items: start; padding: 10px 12px;
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 8px;
}
.binding-kind { padding-top: 2px; }
.binding-body { min-width: 0; }
.binding-head { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.binding-body .io { margin-top: 8px; }
.binding-body .kv { margin: 6px 0; }
.badge-bkind-http { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-bkind-grpc { background: var(--ok-soft); color: var(--ok); }
.badge-bkind-cli { background: var(--warn-soft); color: var(--warn); }
.badge-bkind-event { background: rgba(91,91,230,.10); color: var(--accent); }
.badge-bkind-graphql { background: var(--danger-soft); color: var(--danger); }
.badge-bkind-sdk { background: var(--surface); color: var(--fg-soft); border: 1px solid var(--border); }
.badge-bkind-schedule { background: var(--warn-soft); color: var(--warn); }
.badge-direction-produce { background: var(--accent-soft); color: var(--accent); }
.badge-direction-consume { background: var(--ok-soft); color: var(--ok); }
.badge-gql-query { background: var(--ok-soft); color: var(--ok); }
.badge-gql-mutation { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-gql-subscription { background: var(--warn-soft); color: var(--warn); }
.step-kind-label {
  font-size: 10px; font-weight: 700; letter-spacing: 0.1em;
  padding: 2px 6px; border-radius: 4px;
  background: var(--surface-2); color: var(--muted);
}

/* decisions: markdown bodies */
.md { color: var(--fg); }
.md h1, .md h2, .md h3, .md h4 { letter-spacing: -0.01em; margin: 18px 0 8px; }
.md h1 { font-size: 22px; }
.md h2 { font-size: 18px; border-bottom: 1px solid var(--border); padding-bottom: 4px; }
.md h3 { font-size: 16px; }
.md h4 { font-size: 14px; color: var(--fg-soft); }
.md p { margin: 8px 0; }
.md ul, .md ol { padding-left: 22px; margin: 8px 0; }
.md li { margin: 3px 0; }
.md code { background: var(--code-bg); padding: 1px 6px; border-radius: 4px; }
.md pre {
  background: var(--code-bg); padding: 12px 14px; border-radius: 8px;
  overflow-x: auto; border: 1px solid var(--border);
}
.md pre code { background: transparent; padding: 0; }
.md blockquote {
  border-left: 3px solid var(--accent); padding: 4px 14px;
  margin: 10px 0; background: var(--accent-soft); color: var(--fg-soft); border-radius: 4px;
}
.md table { border-collapse: collapse; margin: 10px 0; font-size: 14px; width: 100%; }
.md th, .md td { padding: 6px 10px; border: 1px solid var(--border); text-align: left; }
.md th { background: var(--surface-2); }
.md hr { border: none; border-top: 1px solid var(--border); margin: 18px 0; }
.md a { color: var(--accent); }

/* decisions cards */
.decision .doc-toc {
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 8px;
  padding: 6px 12px; margin: 6px 0 12px; font-size: 13px;
}
.decision .doc-toc summary {
  cursor: pointer; font-weight: 600; color: var(--fg-soft); padding: 4px 0;
}
.decision .doc-toc ol { padding-left: 22px; }
.decision .doc-toc li.lvl-1 { font-weight: 600; }
.decision .doc-toc li.lvl-2 { padding-left: 8px; }
.decision .doc-toc li.lvl-3 { padding-left: 16px; color: var(--muted); }
.adr-index {
  display: grid; gap: 6px; margin: 12px 0 24px;
}
.adr-index-row {
  display: grid; grid-template-columns: 100px minmax(150px, 1fr) 2fr; gap: 12px;
  padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px;
  background: var(--surface); color: var(--fg);
  transition: border-color .15s;
}
.adr-index-row:hover { border-color: var(--accent); text-decoration: none; }
.adr-index-row .adr-num { color: var(--accent); font-weight: 700; }
.adr-index-row .adr-title { font-weight: 600; }
.adr-index-row .adr-preview { color: var(--muted); font-size: 13px; line-height: 1.4; overflow: hidden; }
@media (max-width: 720px) {
  .adr-index-row { grid-template-columns: 1fr; gap: 4px; }
}

/* work items */
.ch-index { display: grid; gap: 6px; margin: 12px 0 24px; }
.ch-index-row {
  display: grid; grid-template-columns: 120px minmax(0, 1fr) auto auto auto;
  gap: 10px; align-items: center;
  padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px;
  background: var(--surface); color: var(--fg);
  transition: border-color .15s;
}
.ch-index-row:hover { border-color: var(--accent); text-decoration: none; }
.ch-index-row .ch-id { color: var(--muted); font-size: 12px; }
.ch-index-row .ch-title { font-weight: 600; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.work-item, .completion-record { margin-top: 8px; }
.wi-header, .cr-header { display: flex; flex-wrap: wrap; gap: 8px; align-items: baseline; margin-bottom: 8px; }
.wi-header h4, .cr-header h4 { margin: 0; font-size: 15px; }
.wi-meta, .cr-meta { display: flex; flex-wrap: wrap; gap: 4px; }
.change-block {
  margin: 6px 0; padding: 8px 12px;
  background: var(--surface-2); border: 1px solid var(--border); border-radius: 8px;
}
.change-block summary {
  cursor: pointer; font-weight: 600; color: var(--fg-soft); padding: 2px 0;
  font-size: 13px; text-transform: uppercase; letter-spacing: 0.06em;
}
.change-block[open] summary { color: var(--accent); margin-bottom: 6px; }
.change-block p { margin: 6px 0; }
.scope-group { margin: 8px 0; }
.scope-group h4 {
  font-size: 12px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); margin: 4px 0;
}
.change-list { padding-left: 22px; margin: 6px 0; }
.change-list li { margin: 4px 0; }
.completion-record {
  margin-top: 16px; padding-top: 14px; border-top: 1px dashed var(--border);
}
`

const SCRIPT = `
(() => {
  document.documentElement.classList.add('js');

  // Parse "#tab=foo&sec=bar" into { tab, sec }. Defaults to overview tab.
  const parseHash = () => {
    const h = location.hash.replace(/^#/, '');
    const out = { tab: 'overview', sec: '' };
    if (!h) return out;
    if (!h.includes('=')) {
      // Bare anchor like "#models" — resolve the owning tab from the target node.
      const sec = h;
      out.sec = sec;
      const target = document.getElementById(sec);
      if (target) {
        const tab = target.closest('[data-tab]');
        if (tab && tab.getAttribute('data-tab')) out.tab = tab.getAttribute('data-tab');
      } else {
        // Fallback: find the tab whose TOC owns this id.
        const toc = document.querySelector('.toc a[data-sec="' + cssEscape(sec) + '"]');
        if (toc) {
          const tocBox = toc.closest('.toc');
          out.tab = (tocBox && tocBox.getAttribute('data-toc-for')) || 'overview';
        }
      }
      return out;
    }
    for (const part of h.split('&')) {
      const [k, v] = part.split('=');
      if (k === 'tab' && v) out.tab = v;
      if (k === 'sec' && v) out.sec = v;
    }
    return out;
  };

  // Minimal CSS.escape polyfill for older runtimes.
  const cssEscape = (s) => {
    if (window.CSS && CSS.escape) return CSS.escape(s);
    return String(s).replace(/[^a-zA-Z0-9_-]/g, (c) => '\\\\' + c);
  };

  const tabs = Array.from(document.querySelectorAll('[data-tab]'));
  const tabLinks = Array.from(document.querySelectorAll('[data-tab-link]'));
  const tocs = Array.from(document.querySelectorAll('[data-toc-for]'));

  const showTab = (name) => {
    let found = false;
    for (const t of tabs) {
      const match = t.getAttribute('data-tab') === name;
      t.classList.toggle('active', match);
      if (match) found = true;
    }
    if (!found) name = 'overview';
    for (const t of tabs) t.classList.toggle('active', t.getAttribute('data-tab') === name);
    for (const a of tabLinks) a.classList.toggle('active', a.getAttribute('data-tab-link') === name);
    for (const toc of tocs) toc.classList.toggle('active', toc.getAttribute('data-toc-for') === name);
    return name;
  };

  const route = () => {
    const { tab, sec } = parseHash();
    const active = showTab(tab);
    if (sec) {
      const el = document.getElementById(sec);
      if (el) {
        // Scroll without forcing :target focus, so the hash router stays in control.
        requestAnimationFrame(() => el.scrollIntoView({ block: 'start' }));
      }
    } else {
      window.scrollTo(0, 0);
    }
    setActiveLink(active);
  };

  const setActiveLink = (tab) => {
    const toc = document.querySelector('.toc[data-toc-for="' + cssEscape(tab) + '"]');
    if (!toc) return;
    const links = Array.from(toc.querySelectorAll('a[data-sec]'));
    if (!links.length) return;
    const sections = links
      .map((a) => document.getElementById(a.getAttribute('data-sec')))
      .filter(Boolean);
    if (!sections.length) return;
    const TRIGGER = 120;
    const update = () => {
      let current = sections[0];
      for (const s of sections) {
        if (s.getBoundingClientRect().top - TRIGGER <= 0) current = s;
        else break;
      }
      for (const a of links) {
        a.classList.toggle('active', a.getAttribute('data-sec') === current.id);
      }
    };
    update();
    if (window._scrollSpy) window.removeEventListener('scroll', window._scrollSpy);
    window._scrollSpy = () => requestAnimationFrame(update);
    window.addEventListener('scroll', window._scrollSpy, { passive: true });
  };

  window.addEventListener('hashchange', route);
  route();
})();
`

export const renderPage = (site: SiteInput): string => {
  const { scl, decisions, work_items: workItems } = site
  const title = site.title ?? scl.system

  const tabs = [
    renderTab('overview', renderOverviewTab(site)),
    renderTab('scl', renderSclTab(scl)),
    renderTab('decisions', renderDecisionsTab(decisions)),
    renderTab('work-items', renderChangesTab(workItems)),
  ].join('\n')

  const tocs = [
    renderTocFor('overview', [{ id: 'ov-hero', label: 'Overview' }]),
    renderTocFor('scl', sclTocItems(scl)),
    renderTocFor('decisions', decisionsTocItems(decisions)),
    renderTocFor('work-items', changesTocItems(workItems)),
  ].join('\n')

  const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(title)}</title>
<style>${CSS}</style>
</head>
<body>
<header class="app-header">
  <div class="app-header-row">
    <a href="#tab=overview" class="app-title">${esc(title)}</a>
    ${renderTabBar('overview')}
  </div>
</header>
<div class="layout">
  <aside class="toc-wrap">${tocs}</aside>
  <main>${tabs}</main>
</div>
<script>${SCRIPT}</script>
</body>
</html>
`
  return html.replace(/[ \t]+$/gm, '')
}
