/**
 * Layer 3 — pure Render: SclDocument → HtmlDocument
 *
 * Derived from spec/scl.yaml. No I/O, no clock, no env. Deterministic.
 *
 * Section-specific UI with sticky TOC, card layout, and badge/chip primitives
 * reused across sections. Cross-references (emits / errors / interface) become
 * anchor links to the corresponding definition inside the same document.
 */

export const SECTION_KINDS = [
  'standards',
  'vocabulary',
  'models',
  'interfaces',
  'states',
  'invariants',
  'scenarios',
  'permissions',
  'objectives',
  'assurance',
  'user_experience',
] as const

export type SectionKind = (typeof SECTION_KINDS)[number]

export interface SclDocument {
  system: string
  spec_version: string
  annotations?: Record<string, unknown>
  standards?: Record<string, Standard>
  vocabulary?: Record<string, Vocabulary>
  models?: Record<string, Model>
  interfaces?: Record<string, Interface>
  states?: Record<string, StateMachine>
  invariants?: Record<string, Invariant>
  scenarios?: Record<string, Scenario>
  permissions?: Record<string, Permission>
  objectives?: Record<string, Objective>
  assurance?: Record<string, AssuranceObligation>
  user_experience?: UserExperience
}

interface Standard {
  title?: string
  version?: string
  url?: string
  roles?: string[]
  scope?: string
  requirements?: StandardRequirement[]
}

interface StandardRequirement {
  id?: string
  section?: string
  strength?: string
  adoption?: 'required' | 'optional' | 'excluded'
  statement?: string
  reason?: string
  relates_to?: Record<string, string[]>
}

interface UserExperience {
  accessibility?: { standard?: string; level?: string }
  locales?: string[]
  screens?: Record<
    string,
    { route?: string; purpose?: string; interfaces?: string[]; states?: string[] }
  >
  transitions?: Array<{
    from?: string
    to?: string
    trigger?: string
    interface?: string
    external?: boolean
  }>
  requirements?: Array<{
    id?: string
    category?: string
    adoption?: 'required' | 'optional' | 'excluded'
    statement?: string
    reason?: string
    screens?: string[]
    interfaces?: string[]
    standards?: string[]
    scenarios?: string[]
    invariants?: string[]
  }>
}

interface Vocabulary {
  definition?: string
  description?: string
  aliases?: string[]
  context?: string
  not_to_confuse_with?: Array<{ term?: string; reason?: string }>
  annotations?: Record<string, unknown>
}

interface Field {
  type?: unknown
  optional?: boolean
  default?: unknown
  constraints?: unknown[]
  description?: string
  annotations?: Record<string, unknown>
}

interface Model {
  kind?: string
  description?: string
  identity?: string | string[]
  annotations?: Record<string, unknown>
  values?: string[]
  fields?: Record<string, Field>
  payload?: Record<string, Field>
}

interface Interface {
  description?: string
  steps?: string[]
  input?: Record<string, Field>
  output?: Record<string, Field>
  errors?: string[]
  emits?: string[]
  idempotent?: boolean
  read_only?: boolean
  bindings?: Binding[]
  annotations?: Record<string, unknown>
}

type Binding =
  | ({ kind: 'http' } & HttpBinding)
  | ({ kind: 'grpc' } & GrpcBinding)
  | ({ kind: 'cli' } & CliBinding)
  | ({ kind: 'event' } & EventBinding)
  | ({ kind: 'graphql' } & GraphqlBinding)
  | ({ kind: 'sdk' } & SdkBinding)
  | ({ kind: 'schedule' } & ScheduleBinding)
  | { kind: string; [k: string]: unknown }

interface HttpBinding {
  method?: string
  path?: string
  successful_status_codes?: string[]
  request_form?: 'body' | 'query' | 'form'
  headers?: Record<string, Field>
  description?: string
}

interface GrpcBinding {
  service?: string
  method?: string
  streaming?: 'unary' | 'client' | 'server' | 'bidi'
  description?: string
}

interface CliBinding {
  command?: string
  args?: Array<{
    name?: string
    position?: number
    short?: string
    required?: boolean
    repeatable?: boolean
  }>
  flags?: Array<{ name?: string; short?: string; required?: boolean; repeatable?: boolean }>
  stdin?: unknown
  stdout?: unknown
  exit_codes?: Record<string, number>
  description?: string
}

interface EventBinding {
  channel?: string
  direction?: 'produce' | 'consume'
  delivery?: 'at_most_once' | 'at_least_once' | 'exactly_once'
  ordering?: 'none' | 'per_key' | 'global'
  partition_key?: string
  description?: string
}

interface GraphqlBinding {
  operation?: 'query' | 'mutation' | 'subscription'
  field?: string
  description?: string
}

interface SdkBinding {
  function?: string
  description?: string
}

interface ScheduleBinding {
  cron?: string
  every?: string
  description?: string
}

interface StateMachine {
  description?: string
  annotations?: Record<string, unknown>
  target?: string
  initial?: string
  terminal?: string[]
  transitions?: Array<{
    from?: string
    to?: string
    event?: string
    on?: string
    guard?: unknown
    effect?: string[]
  }>
}

interface Invariant {
  description?: string
  annotations?: Record<string, unknown>
  target?: string
  assuming?: unknown
  always?: unknown
  never?: unknown
  eventually?: unknown
  within?: string
  severity?: string
}

interface Scenario {
  description?: string
  annotations?: Record<string, unknown>
  tags?: string[]
  steps?: string[]
  where?: Array<Record<string, unknown>>
}

interface Permission {
  description?: string
  annotations?: Record<string, unknown>
  actor?: string
  action?: string
  resource?: string
  allow_when?: unknown
  deny_when?: unknown
}

interface Objective {
  kind?: string
  description?: string
  annotations?: Record<string, unknown>
  [k: string]: unknown
}

interface AssuranceObligation {
  claim?: string
  risk?: string
  risk_level?: string
  derived_from?: Record<string, string[]>
  acceptance?: unknown
  evidence?: Record<string, EvidenceRequirement>
  approval?: {
    when?: string[]
    role?: string
    decision_record?: boolean
  }
  annotations?: Record<string, unknown>
}

interface EvidenceRequirement {
  kind?: string
  producer?: string
  evaluation?: string
  environments?: string[]
  recheck?: string
  covers?: Record<string, string[]>
  procedure?: string
  oracle?: string
}

const referenceAnchor = (section: string, name: string): string | undefined => {
  const prefixes: Record<string, string> = {
    vocabulary: 'vocab',
    models: 'model',
    interfaces: 'iface',
    states: 'state',
    invariants: 'inv',
    scenarios: 'scn',
    permissions: 'perm',
    objectives: 'obj',
    standards: 'std',
    assurance: 'assurance',
  }
  const prefix = prefixes[section]
  return prefix ? `#${prefix}-${slug(name)}` : undefined
}

const renderNamedReferences = (refs?: Record<string, string[]>): string => {
  if (!refs) return ''
  return Object.entries(refs)
    .map(([section, names]) => {
      const values = names
        .map((name) => {
          const href = referenceAnchor(section, name)
          return href ? link(href, name, 'ref') : chip(name)
        })
        .join(' ')
      return `<div class="reference-row"><span class="reference-label">${esc(section)}</span>${values}</div>`
    })
    .join('')
}

// ─── section: standards ────────────────────────────────────────────

const renderStandards = (standards: Record<string, Standard>): string => {
  const cards = Object.entries(standards)
    .map(([name, standard]) => {
      const requirements = (standard.requirements ?? [])
        .map((requirement) => {
          const adoption = requirement.adoption ?? 'required'
          return `<article class="requirement" id="req-${esc(slug(requirement.id ?? ''))}">
            <header>
              <code class="name">${esc(requirement.id)}</code>
              ${badge(adoption, `adoption-${adoption}`)}
              ${requirement.strength ? badge(requirement.strength, 'strength') : ''}
              ${requirement.section ? chip(requirement.section, 'hint') : ''}
            </header>
            ${requirement.statement ? `<p>${esc(requirement.statement)}</p>` : ''}
            ${requirement.reason ? `<p class="exclusion-reason"><strong>reason:</strong> ${esc(requirement.reason)}</p>` : ''}
            ${renderNamedReferences(requirement.relates_to)}
          </article>`
        })
        .join('')
      return `<article class="card standard" id="std-${esc(slug(name))}">
        <header><h3>${esc(name)}</h3>${standard.version ? badge(standard.version, 'version') : ''}</header>
        <p class="desc"><strong>${esc(standard.title)}</strong>${standard.scope ? ` — ${esc(standard.scope)}` : ''}</p>
        <dl class="kv">
          ${standard.url ? kvRow('source', `<a href="${esc(standard.url)}">${esc(standard.url)}</a>`) : ''}
          ${standard.roles?.length ? kvRow('roles', standard.roles.map((role) => chip(role)).join(' ')) : ''}
        </dl>
        <div class="requirements">${requirements}</div>
      </article>`
    })
    .join('')
  const requirementCount = Object.values(standards).reduce(
    (total, standard) => total + (standard.requirements?.length ?? 0),
    0,
  )
  return wrapSection(
    'standards',
    'Standards',
    `適用する外部標準と規範要件。${requirementCount} requirements。採用方針であり実装状態ではない。`,
    `<div class="cards">${cards}</div>`,
    Object.keys(standards).length,
  )
}

// ─── section: user experience ──────────────────────────────────────

const renderUserExperience = (ux: UserExperience): string => {
  const screens = Object.entries(ux.screens ?? {})
    .map(
      ([name, screen]) => `<article class="card" id="screen-${esc(slug(name))}">
        <header><h3>${esc(name)}</h3>${screen.route ? chip(screen.route, 'hint') : ''}</header>
        ${screen.purpose ? `<p class="desc">${esc(screen.purpose)}</p>` : ''}
        ${
          screen.interfaces?.length
            ? `<div class="reference-row"><span class="reference-label">interfaces</span>${screen.interfaces
                .map((item) => link(`#iface-${slug(item)}`, item, 'iface-ref'))
                .join(' ')}</div>`
            : ''
        }
        ${screen.states?.length ? `<div class="chip-row">${screen.states.map((state) => chip(state)).join(' ')}</div>` : ''}
      </article>`,
    )
    .join('')

  const transitions = (ux.transitions ?? [])
    .map(
      (transition) => `<tr>
        <td>${transition.from ? link(`#screen-${slug(transition.from)}`, transition.from) : '<span class="muted">system</span>'}</td>
        <td><code>${esc(transition.trigger)}</code></td>
        <td>${
          transition.to
            ? link(`#screen-${slug(transition.to)}`, transition.to)
            : transition.external
              ? badge('external', 'optional')
              : ''
        }</td>
        <td>${transition.interface ? link(`#iface-${slug(transition.interface)}`, transition.interface, 'iface-ref') : ''}</td>
      </tr>`,
    )
    .join('')

  const requirements = (ux.requirements ?? [])
    .map((requirement) => {
      const refs: Record<string, string[]> = {}
      if (requirement.interfaces) refs.interfaces = requirement.interfaces
      if (requirement.standards) refs.standards = requirement.standards
      if (requirement.scenarios) refs.scenarios = requirement.scenarios
      if (requirement.invariants) refs.invariants = requirement.invariants
      return `<article class="requirement" id="ux-${esc(slug(requirement.id ?? ''))}">
        <header>
          <code class="name">${esc(requirement.id)}</code>
          ${badge(requirement.adoption ?? 'required', `adoption-${requirement.adoption ?? 'required'}`)}
          ${requirement.category ? badge(requirement.category, 'category') : ''}
        </header>
        ${requirement.statement ? `<p>${esc(requirement.statement)}</p>` : ''}
        ${
          requirement.screens?.length
            ? `<div class="reference-row"><span class="reference-label">screens</span>${requirement.screens
                .map((screen) => link(`#screen-${slug(screen)}`, screen))
                .join(' ')}</div>`
            : ''
        }
        ${renderNamedReferences(refs)}
      </article>`
    })
    .join('')

  const metadata = `<dl class="kv">
    ${
      ux.accessibility
        ? kvRow(
            'accessibility',
            `${link(`#std-${slug(ux.accessibility.standard ?? '')}`, ux.accessibility.standard)} ${badge(ux.accessibility.level)}`,
          )
        : ''
    }
    ${ux.locales?.length ? kvRow('locales', ux.locales.map((locale) => chip(locale)).join(' ')) : ''}
  </dl>`

  return wrapSection(
    'user_experience',
    'User Experience',
    'ブラウザ画面、画面遷移、セキュリティ・プライバシー・アクセシビリティ要件。',
    `${metadata}
    <div class="group"><h3 class="grp-title">Screens <span class="count">${Object.keys(ux.screens ?? {}).length}</span></h3><div class="cards">${screens}</div></div>
    <div class="group"><h3 class="grp-title">Transitions <span class="count">${ux.transitions?.length ?? 0}</span></h3><table class="fields"><thead><tr><th>From</th><th>Trigger</th><th>To</th><th>Interface</th></tr></thead><tbody>${transitions}</tbody></table></div>
    <div class="group"><h3 class="grp-title">Requirements <span class="count">${ux.requirements?.length ?? 0}</span></h3><div class="requirements">${requirements}</div></div>`,
    Object.keys(ux.screens ?? {}).length,
  )
}

// ─── primitives ────────────────────────────────────────────────────

const esc = (s: unknown): string =>
  String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')

const slug = (s: string): string =>
  s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')

const isObj = (v: unknown): v is Record<string, unknown> =>
  v !== null && typeof v === 'object' && !Array.isArray(v)

const chip = (text: unknown, kind = ''): string =>
  `<span class="chip${kind ? ` chip-${kind}` : ''}">${esc(text)}</span>`

const link = (href: string, text: unknown, kind = ''): string =>
  `<a class="chip${kind ? ` chip-${kind}` : ''}" href="${esc(href)}">${esc(text)}</a>`

const badge = (text: unknown, kind = ''): string =>
  `<span class="badge${kind ? ` badge-${kind}` : ''}">${esc(text)}</span>`

const typeText = (t: unknown): string =>
  typeof t === 'string' ? t : t === undefined || t === null ? 'unknown' : JSON.stringify(t)

const kvRow = (k: string, v: string): string => `<dt>${esc(k)}</dt><dd>${v}</dd>`

// ─── generic value rendering (used inside scenarios / annotations) ─

const renderValue = (v: unknown): string => {
  if (v === null || v === undefined) return '<span class="muted">—</span>'
  if (typeof v === 'boolean' || typeof v === 'number') return `<code>${v}</code>`
  if (typeof v === 'string') return `<span class="text">${esc(v)}</span>`
  if (Array.isArray(v)) {
    if (v.length === 0) return '<span class="muted">[]</span>'
    if (v.every((x) => typeof x === 'string' || typeof x === 'number'))
      return v.map((x) => chip(x)).join(' ')
    return `<ul class="vlist">${v.map((x) => `<li>${renderValue(x)}</li>`).join('')}</ul>`
  }
  if (isObj(v)) {
    const pairs = Object.entries(v)
      .map(([k, val]) => kvRow(k, renderValue(val)))
      .join('')
    return `<dl class="kv">${pairs}</dl>`
  }
  return esc(String(v))
}

// ─── constraint / annotation chips ─────────────────────────────────

const humanConstraint = (c: unknown): string => {
  if (typeof c === 'string') return c.replace(/_/g, ' ')
  if (isObj(c)) {
    return Object.entries(c)
      .map(([k, v]) => {
        const label = k.replace(/_/g, ' ')
        const val = isObj(v) || Array.isArray(v) ? JSON.stringify(v) : String(v)
        return `${label}: ${val}`
      })
      .join(', ')
  }
  return String(c)
}

const renderConstraints = (cs: unknown): string =>
  Array.isArray(cs) ? cs.map((c) => chip(humanConstraint(c), 'constraint')).join(' ') : ''

const renderAnnotations = (ann: unknown): string => {
  if (!isObj(ann)) return ''
  return Object.entries(ann)
    .filter(([, v]) => v !== false)
    .map(([k, v]) => {
      const label = k.replace(/_/g, ' ')
      const text =
        v === true ? label : `${label}: ${isObj(v) || Array.isArray(v) ? JSON.stringify(v) : v}`
      return chip(text, 'annotation')
    })
    .join(' ')
}

// ─── fields / IO tables ────────────────────────────────────────────

const fieldRow = (name: string, f: Field): string => {
  const presence = f.optional ? badge('optional', 'optional') : badge('required', 'required')
  const def =
    f.default !== undefined
      ? chip(
          `default: ${isObj(f.default) || Array.isArray(f.default) ? JSON.stringify(f.default) : f.default}`,
          'default',
        )
      : ''
  const meta = [renderConstraints(f.constraints), renderAnnotations(f.annotations), def]
    .filter(Boolean)
    .join(' ')
  return `<tr>
    <td><code class="name">${esc(name)}</code></td>
    <td><code class="type">${esc(typeText(f.type))}</code></td>
    <td>${presence}</td>
    <td>${meta || '<span class="muted">—</span>'}</td>
    <td>${f.description ? esc(f.description) : ''}</td>
  </tr>`
}

const fieldsTable = (fields: Record<string, Field>): string => {
  const rows = Object.entries(fields)
    .map(([n, f]) => fieldRow(n, f))
    .join('')
  return `<table class="fields">
    <thead><tr><th>Field</th><th>Type</th><th>Presence</th><th>Constraints / Annotations</th><th>Description</th></tr></thead>
    <tbody>${rows}</tbody>
  </table>`
}

const ioTable = (io: Record<string, Field>, label: string): string => {
  const rows = Object.entries(io)
    .map(
      ([k, f]) => `<tr>
    <td><code class="name">${esc(k)}</code></td>
    <td><code class="type">${esc(typeText(f.type))}</code></td>
    <td>${f.optional ? badge('optional', 'optional') : badge('required', 'required')}</td>
    <td>${f.description ? esc(f.description) : ''}</td>
  </tr>`,
    )
    .join('')
  return `<div class="io">
    <div class="label">${esc(label)}</div>
    <table class="fields">
      <thead><tr><th>Slot</th><th>Type</th><th>Presence</th><th>Description</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>
  </div>`
}

// ─── logical expression tree (invariants / permissions) ───────────

const renderExpression = (v: unknown): string => {
  if (typeof v === 'string') return `<code class="expr">${esc(v)}</code>`
  if (Array.isArray(v)) {
    if (v.every((x) => typeof x === 'string'))
      return `<ul class="expr-list">${v
        .map((x) => `<li><code class="expr">${esc(x)}</code></li>`)
        .join('')}</ul>`
    return `<ul class="expr-list">${v.map((x) => `<li>${renderExpression(x)}</li>`).join('')}</ul>`
  }
  if (isObj(v)) {
    return Object.entries(v)
      .map(([op, operand]) => renderExprOp(op, operand))
      .join('')
  }
  return `<code>${esc(String(v))}</code>`
}

/** Render one operator/operand. Special-cases forall/exists quantifiers and atomic predicates. */
const renderExprOp = (op: string, operand: unknown): string => {
  // Quantifier: forall / exists with { in, satisfies }
  if (
    (op === 'forall' || op === 'exists') &&
    isObj(operand) &&
    'in' in operand &&
    'satisfies' in operand
  ) {
    const sym = op === 'forall' ? '∀' : '∃'
    return `<div class="expr-quant">
      <span class="expr-quant-sym">${sym} x ∈</span>
      ${renderExpression(operand.in)}
      <span class="expr-quant-sym">:</span>
      ${renderExpression(operand.satisfies)}
    </div>`
  }
  // Single-field atomic predicates: render inline
  if ((op === 'exists' || op === 'not_exists') && typeof operand === 'string') {
    const label = op === 'exists' ? 'EXISTS' : 'NOT EXISTS'
    return `<code class="expr">${label}(${esc(operand)})</code>`
  }
  if ((op === 'count' || op === 'len') && (typeof operand === 'string' || isObj(operand))) {
    const inner = typeof operand === 'string' ? esc(operand) : renderExpression(operand)
    return `<code class="expr">${op.toUpperCase()}(${inner})</code>`
  }
  // Default: operator block with recursive operand
  return `<div class="expr-op">
    <div class="expr-op-label">${esc(op.toUpperCase())}</div>
    ${renderExpression(operand)}
  </div>`
}

// ─── section: vocabulary ────────────────────────────────────────────────

const renderVocab = (entries: Record<string, Vocabulary>): string => {
  const items = Object.entries(entries)
    .map(([name, v]) => {
      const def = v.definition ?? v.description ?? ''
      const ctx = v.context ? badge(v.context, 'context') : ''
      const aliases = v.aliases?.length
        ? v.aliases.map((a) => chip(a, 'alias')).join(' ')
        : '<span class="muted">—</span>'
      const ann = renderAnnotations(v.annotations)
      const ntcw = v.not_to_confuse_with?.length
        ? `<div class="ntcw">
        <div class="ntcw-label">do not confuse with</div>
        <ul>${v.not_to_confuse_with
          .map(
            (n) =>
              `<li><strong>${esc(n.term ?? '')}</strong> — <span class="muted">${esc(n.reason ?? '')}</span></li>`,
          )
          .join('')}</ul>
      </div>`
        : ''
      return `<article class="vocab-entry" id="vocab-${esc(slug(name))}">
      <div class="vocab-name"><code class="name">${esc(name)}</code>${ctx}</div>
      <div class="vocab-body">
        ${def ? `<p class="def">${esc(def)}</p>` : ''}
        <div class="aliases"><span class="aliases-label">aliases</span> ${aliases}</div>
        ${ann ? `<dl class="kv">${kvRow('annotations', ann)}</dl>` : ''}
        ${ntcw}
      </div>
    </article>`
    })
    .join('\n')
  return wrapSection(
    'vocabulary',
    'Vocabulary',
    '本仕様で使うドメイン用語の定義。PascalCase が正準名、aliases は実装上の別表記。',
    `<div class="vocab-list">${items}</div>`,
    Object.keys(entries).length,
  )
}

// ─── section: models ───────────────────────────────────────────────

const MODEL_KIND_ORDER = ['entity', 'value_object', 'enum', 'event', 'error']

const renderModel = (name: string, m: Model): string => {
  const kind = m.kind ?? 'unknown'
  const desc = m.description ? `<p class="desc">${esc(m.description)}</p>` : ''
  const metaRows: string[] = []
  if (m.identity) {
    const id = Array.isArray(m.identity) ? m.identity.join(', ') : m.identity
    metaRows.push(kvRow('identity', `<code>${esc(id)}</code>`))
  }
  const ann = renderAnnotations(m.annotations)
  if (ann) metaRows.push(kvRow('annotations', ann))
  let body = ''
  if (kind === 'enum') {
    body = `<div class="enum-vals">${(m.values ?? []).map((v) => chip(v, 'enum')).join(' ')}</div>`
  } else if (m.fields) {
    body = fieldsTable(m.fields)
  } else if (m.payload) {
    body = `<div class="label">Payload</div>${fieldsTable(m.payload)}`
  } else {
    body = '<p class="muted">(no fields)</p>'
  }
  return `<article class="card" id="model-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${badge(kind.replace(/_/g, ' '), `kind-${kind}`)}</header>
    ${desc}
    ${metaRows.length ? `<dl class="kv">${metaRows.join('')}</dl>` : ''}
    ${body}
  </article>`
}

const renderModels = (models: Record<string, Model>): string => {
  const grouped = new Map<string, Array<[string, Model]>>()
  for (const [n, m] of Object.entries(models)) {
    const k = m.kind ?? 'unknown'
    const bucket = grouped.get(k) ?? []
    bucket.push([n, m])
    grouped.set(k, bucket)
  }
  const ordered = [
    ...MODEL_KIND_ORDER.filter((k) => grouped.has(k)),
    ...[...grouped.keys()].filter((k) => !MODEL_KIND_ORDER.includes(k)),
  ]
  const groups = ordered
    .map((kind) => {
      const entries = grouped.get(kind) ?? []
      const items = entries.map(([n, m]) => renderModel(n, m)).join('\n')
      return `<div class="group">
      <h3 class="grp-title">${esc(kind.replace(/_/g, ' '))} <span class="count">${entries.length}</span></h3>
      <div class="cards">${items}</div>
    </div>`
    })
    .join('\n')
  return wrapSection('models', 'Models', '', groups, Object.keys(models).length)
}

// ─── section: interfaces ───────────────────────────────────────────

const renderBindingBody = (b: Binding): string => {
  switch (b.kind) {
    case 'http': {
      const h = b as HttpBinding
      const head: string[] = []
      if (h.method) head.push(badge(h.method, `method-${h.method.toLowerCase()}`))
      if (h.path) head.push(`<code class="path">${esc(h.path)}</code>`)
      if (h.successful_status_codes?.length)
        head.push(chip(`response: ${h.successful_status_codes.join(', ')}`, 'hint'))
      if (h.request_form) head.push(chip(`form: ${h.request_form}`, 'hint'))
      const headers = h.headers ? ioTable(h.headers, 'HTTP Headers') : ''
      return `<div class="binding-head">${head.join(' ')}</div>${headers}`
    }
    case 'grpc': {
      const g = b as GrpcBinding
      const head: string[] = []
      if (g.service && g.method)
        head.push(`<code class="path">${esc(g.service)}.${esc(g.method)}</code>`)
      else if (g.method) head.push(`<code class="path">${esc(g.method)}</code>`)
      if (g.streaming) head.push(chip(`streaming: ${g.streaming}`, 'hint'))
      return `<div class="binding-head">${head.join(' ')}</div>`
    }
    case 'cli': {
      const c = b as CliBinding
      const head: string[] = []
      if (c.command) head.push(`<code class="path">${esc(c.command)}</code>`)
      const out: string[] = [`<div class="binding-head">${head.join(' ')}</div>`]
      if (c.args?.length) {
        const rows = c.args
          .map(
            (a) => `<tr>
            <td><code class="name">${esc(a.name ?? '')}</code></td>
            <td>${a.position !== undefined ? chip(`pos ${a.position}`, 'hint') : ''}</td>
            <td>${a.required ? badge('required', 'required') : badge('optional', 'optional')}</td>
            <td>${a.repeatable ? chip('repeatable', 'hint') : ''}</td>
          </tr>`,
          )
          .join('')
        out.push(
          `<div class="io"><div class="label">Args (positional)</div><table class="fields"><thead><tr><th>Name</th><th>Position</th><th>Presence</th><th>Note</th></tr></thead><tbody>${rows}</tbody></table></div>`,
        )
      }
      if (c.flags?.length) {
        const rows = c.flags
          .map(
            (f) => `<tr>
            <td><code class="name">--${esc(f.name ?? '')}</code></td>
            <td>${f.short ? `<code>-${esc(f.short)}</code>` : ''}</td>
            <td>${f.required ? badge('required', 'required') : badge('optional', 'optional')}</td>
            <td>${f.repeatable ? chip('repeatable', 'hint') : ''}</td>
          </tr>`,
          )
          .join('')
        out.push(
          `<div class="io"><div class="label">Flags</div><table class="fields"><thead><tr><th>Long</th><th>Short</th><th>Presence</th><th>Note</th></tr></thead><tbody>${rows}</tbody></table></div>`,
        )
      }
      const meta: string[] = []
      if (c.stdin !== undefined) meta.push(kvRow('stdin', `<code>${esc(typeText(c.stdin))}</code>`))
      if (c.stdout !== undefined)
        meta.push(kvRow('stdout', `<code>${esc(typeText(c.stdout))}</code>`))
      if (c.exit_codes && Object.keys(c.exit_codes).length) {
        const codes = Object.entries(c.exit_codes)
          .map(([k, v]) => `${esc(k)}: <code>${v}</code>`)
          .join(' · ')
        meta.push(kvRow('exit codes', codes))
      }
      if (meta.length) out.push(`<dl class="kv">${meta.join('')}</dl>`)
      return out.join('')
    }
    case 'event': {
      const e = b as EventBinding
      const head: string[] = []
      if (e.channel) head.push(`<code class="path">${esc(e.channel)}</code>`)
      if (e.direction) head.push(badge(e.direction, `direction-${e.direction}`))
      if (e.delivery) head.push(chip(`delivery: ${e.delivery}`, 'hint'))
      if (e.ordering) head.push(chip(`ordering: ${e.ordering}`, 'hint'))
      const meta = e.partition_key
        ? `<dl class="kv">${kvRow('partition key', `<code>${esc(e.partition_key)}</code>`)}</dl>`
        : ''
      return `<div class="binding-head">${head.join(' ')}</div>${meta}`
    }
    case 'graphql': {
      const g = b as GraphqlBinding
      const head: string[] = []
      if (g.operation) head.push(badge(g.operation, `gql-${g.operation}`))
      if (g.field) head.push(`<code class="path">${esc(g.field)}</code>`)
      return `<div class="binding-head">${head.join(' ')}</div>`
    }
    case 'sdk': {
      const s = b as SdkBinding
      return s.function
        ? `<div class="binding-head"><code class="path">${esc(s.function)}</code></div>`
        : ''
    }
    default: {
      // unknown kind — fall back to generic key-value dump
      const rest = Object.fromEntries(
        Object.entries(b as Record<string, unknown>).filter(
          ([k]) => k !== 'kind' && k !== 'description',
        ),
      )
      return Object.keys(rest).length
        ? `<dl class="kv">${Object.entries(rest)
            .map(([k, v]) => kvRow(k, renderValue(v)))
            .join('')}</dl>`
        : ''
    }
  }
}

const renderBinding = (b: Binding): string => {
  const kind = b.kind ?? 'unknown'
  const desc = (b as { description?: string }).description
    ? `<p class="desc">${esc((b as { description?: string }).description!)}</p>`
    : ''
  return `<div class="binding binding-${esc(kind)}">
    <div class="binding-kind">${badge(kind, `bkind-${kind}`)}</div>
    <div class="binding-body">${desc}${renderBindingBody(b)}</div>
  </div>`
}

const renderInterface = (name: string, i: Interface): string => {
  const readOnly = i.read_only ? badge('read-only', 'readonly') : ''
  const idempotent = i.idempotent ? badge('idempotent', 'idempotent') : ''
  const desc = i.description ? `<p class="desc">${esc(i.description)}</p>` : ''
  const stepTpl = i.steps?.length
    ? i.steps
        .map(
          (s) =>
            `<div class="iface-step"><span class="step-kind-label">STEP</span> <code class="step-tpl">${esc(s)}</code></div>`,
        )
        .join('\n')
    : ''
  const blocks: string[] = []
  if (i.input) blocks.push(ioTable(i.input, 'Input'))
  if (i.output) blocks.push(ioTable(i.output, 'Output'))
  if (i.errors?.length) {
    const chips = i.errors.map((e) => link(`#model-${slug(e)}`, e, 'error-ref')).join(' ')
    blocks.push(
      `<div class="io"><div class="label">Errors</div><div class="chip-row">${chips}</div></div>`,
    )
  }
  if (i.emits?.length) {
    const chips = i.emits.map((e) => link(`#model-${slug(e)}`, e, 'event-ref')).join(' ')
    blocks.push(
      `<div class="io"><div class="label">Emits</div><div class="chip-row">${chips}</div></div>`,
    )
  }
  if (i.bindings?.length) {
    const items = i.bindings.map((b) => renderBinding(b)).join('\n')
    blocks.push(
      `<div class="io"><div class="label">Bindings (${i.bindings.length})</div><div class="bindings">${items}</div></div>`,
    )
  }
  const ann = renderAnnotations(i.annotations)
  if (ann) blocks.unshift(`<dl class="kv">${kvRow('annotations', ann)}</dl>`)
  return `<article class="card" id="iface-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${readOnly}${idempotent}</header>
    ${desc}
    ${stepTpl}
    ${blocks.join('\n')}
  </article>`
}

const renderInterfaces = (ifaces: Record<string, Interface>): string => {
  const cards = Object.entries(ifaces)
    .map(([n, i]) => renderInterface(n, i))
    .join('\n')
  return wrapSection(
    'interfaces',
    'Interfaces',
    '外部との契約。入力・出力・エラー・発行イベントと、それを露出する複数のトランスポート（bindings）。',
    `<div class="cards">${cards}</div>`,
    Object.keys(ifaces).length,
  )
}

// ─── section: state machines ───────────────────────────────────────

const renderStateMachine = (name: string, sm: StateMachine): string => {
  const desc = sm.description ? `<p class="desc">${esc(sm.description)}</p>` : ''
  const transitions = sm.transitions ?? []
  const states = new Set<string>()
  const events = new Set<string>()
  for (const t of transitions) {
    if (t.from) states.add(t.from)
    if (t.to) states.add(t.to)
    if (t.event) events.add(t.event)
    if (t.on) events.add(t.on)
  }
  if (sm.initial) states.add(sm.initial)
  for (const s of sm.terminal ?? []) states.add(s)
  const terminal = new Set(sm.terminal ?? [])
  const stateChips = [...states]
    .map((s) => chip(s, s === sm.initial ? 'initial' : terminal.has(s) ? 'terminal' : ''))
    .join(' ')
  const eventChips = [...events].map((e) => chip(e)).join(' ')
  const metaRows: string[] = []
  if (sm.target) metaRows.push(kvRow('target', `<code>${esc(sm.target)}</code>`))
  if (sm.initial) metaRows.push(kvRow('initial', chip(sm.initial, 'initial')))
  if (sm.terminal?.length)
    metaRows.push(kvRow('terminal', sm.terminal.map((s) => chip(s, 'terminal')).join(' ')))
  const ann = renderAnnotations(sm.annotations)
  if (ann) metaRows.push(kvRow('annotations', ann))
  const trRows = transitions
    .map(
      (t) => `<tr>
    <td><code>${esc(t.from)}</code></td>
    <td><code>${esc(t.event ?? t.on)}</code></td>
    <td><code>${esc(t.to)}</code></td>
    <td>${
      t.guard !== undefined
        ? `<code class="expr">${esc(typeof t.guard === 'string' ? t.guard : JSON.stringify(t.guard))}</code>`
        : ''
    }</td>
    <td>${t.effect?.length ? t.effect.map((e) => chip(e, 'event-ref')).join(' ') : ''}</td>
  </tr>`,
    )
    .join('')
  return `<article class="card" id="state-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3></header>
    ${desc}
    ${metaRows.length ? `<dl class="kv">${metaRows.join('')}</dl>` : ''}
    <div class="sm-row"><div class="label">states (${states.size})</div><div>${stateChips || '<span class="muted">—</span>'}</div></div>
    <div class="sm-row"><div class="label">events (${events.size})</div><div>${eventChips || '<span class="muted">—</span>'}</div></div>
    <div class="sub">
      <div class="label">Transitions</div>
      <table class="fields"><thead><tr><th>From</th><th>On</th><th>To</th><th>Guard</th><th>Effect</th></tr></thead><tbody>${trRows}</tbody></table>
    </div>
  </article>`
}

const renderStates = (sms: Record<string, StateMachine>): string => {
  const cards = Object.entries(sms)
    .map(([n, sm]) => renderStateMachine(n, sm))
    .join('\n')
  return wrapSection(
    'states',
    'States',
    '',
    `<div class="cards">${cards}</div>`,
    Object.keys(sms).length,
  )
}

// ─── section: invariants ───────────────────────────────────────────

const renderInvariant = (name: string, p: Invariant): string => {
  const desc = p.description ? `<p class="desc">${esc(p.description.trim())}</p>` : ''
  const ann = renderAnnotations(p.annotations)
  const metaRows = [
    p.target ? kvRow('target', `<code>${esc(p.target)}</code>`) : '',
    ann ? kvRow('annotations', ann) : '',
  ]
    .filter(Boolean)
    .join('')
  const meta = metaRows ? `<dl class="kv">${metaRows}</dl>` : ''
  const severity = p.severity ? badge(p.severity, `severity-${p.severity}`) : ''
  const clauses: string[] = []
  if (p.assuming !== undefined)
    clauses.push(
      `<div class="clause clause-assuming"><div class="clause-label muted">assuming (precondition)</div>${renderExpression(p.assuming)}</div>`,
    )
  if (p.always !== undefined)
    clauses.push(
      `<div class="clause clause-always"><div class="clause-label ok">always (invariant)</div>${renderExpression(p.always)}</div>`,
    )
  if (p.never !== undefined)
    clauses.push(
      `<div class="clause clause-never"><div class="clause-label danger">never (forbidden)</div>${renderExpression(p.never)}</div>`,
    )
  if (p.eventually !== undefined) {
    const within = p.within
      ? ` <span class="within">within <code>${esc(p.within)}</code></span>`
      : ''
    clauses.push(
      `<div class="clause clause-eventually"><div class="clause-label accent">eventually (liveness)${within}</div>${renderExpression(p.eventually)}</div>`,
    )
  }
  return `<article class="card" id="inv-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${severity}</header>
    ${desc}
    ${meta}
    ${clauses.join('\n')}
  </article>`
}

const renderInvariants = (props: Record<string, Invariant>): string => {
  const cards = Object.entries(props)
    .map(([n, p]) => renderInvariant(n, p))
    .join('\n')
  return wrapSection(
    'invariants',
    'Invariants',
    '入力に依らず常に成り立つ不変条件、または決して起きてはならない事象。',
    `<div class="cards">${cards}</div>`,
    Object.keys(props).length,
  )
}

// ─── section: scenarios ────────────────────────────────────────────

/**
 * Cross-reference index: declaration name → in-document anchor.
 * Used to linkify known interface / model (event / error) names that
 * appear as quoted tokens inside a scenario's natural-language steps.
 */
const buildXref = (scl: SclDocument): Map<string, string> => {
  const idx = new Map<string, string>()
  for (const n of Object.keys(scl.interfaces ?? {})) idx.set(n, `#iface-${slug(n)}`)
  for (const n of Object.keys(scl.models ?? {})) idx.set(n, `#model-${slug(n)}`)
  return idx
}

/**
 * Render one natural-language step. Quoted tokens (`"..."`) are highlighted;
 * a quoted token that matches a known declaration becomes a cross-ref link.
 */
const renderStepText = (raw: string, xref: Map<string, string>): string =>
  esc(raw).replace(/&quot;(.*?)&quot;/g, (_m, inner: string) => {
    const href = xref.get(inner)
    return href
      ? `<a class="chip chip-ref" href="${esc(href)}">${inner}</a>`
      : `<code class="step-arg">${inner}</code>`
  })

/** Render the optional `where` data table (parameterization rows). */
const renderWhere = (rows: Array<Record<string, unknown>>): string => {
  const cols = Object.keys(rows[0] ?? {})
  if (!cols.length) return ''
  const head = cols.map((c) => `<th>${esc(c)}</th>`).join('')
  const body = rows
    .map((r) => `<tr>${cols.map((c) => `<td>${renderValue(r[c])}</td>`).join('')}</tr>`)
    .join('')
  return `<div class="sub">
    <div class="label">Where</div>
    <table class="fields"><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>
  </div>`
}

const renderScenario = (name: string, s: Scenario, xref: Map<string, string>): string => {
  const tags = s.tags?.length ? s.tags.map((t) => chip(t, 'tag')).join(' ') : ''
  const desc = s.description ? `<p class="desc">${esc(s.description)}</p>` : ''
  const ann = renderAnnotations(s.annotations)
  const steps = (s.steps ?? [])
    .map((st) => `<li class="scn-step">${renderStepText(String(st), xref)}</li>`)
    .join('')
  const where = s.where?.length ? renderWhere(s.where) : ''
  return `<article class="card scenario" id="scn-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${tags}</header>
    ${desc}
    ${ann ? `<dl class="kv">${kvRow('annotations', ann)}</dl>` : ''}
    <ol class="scn-steps">${steps}</ol>
    ${where}
  </article>`
}

const renderScenarios = (scns: Record<string, Scenario>, xref: Map<string, string>): string => {
  const cards = Object.entries(scns)
    .map(([n, s]) => renderScenario(n, s, xref))
    .join('\n')
  return wrapSection(
    'scenarios',
    'Scenarios',
    '受け入れ例。自然文ステップで書かれた振る舞いの具体例。引用された既知の名前は定義へリンクする。',
    `<div class="cards">${cards}</div>`,
    Object.keys(scns).length,
  )
}

// ─── section: permissions ──────────────────────────────────────────

const renderPermission = (name: string, p: Permission): string => {
  const desc = p.description ? `<p class="desc">${esc(p.description)}</p>` : ''
  const triple = (
    [
      ['actor', p.actor],
      ['action', p.action],
      ['resource', p.resource],
    ] as const
  )
    .map(([k, v]) =>
      kvRow(k, v ? `<code class="name">${esc(v)}</code>` : '<span class="muted">—</span>'),
    )
    .join('')
  const clauses: string[] = []
  if (p.allow_when !== undefined)
    clauses.push(
      `<div class="clause"><div class="clause-label ok">allow when</div>${renderExpression(p.allow_when)}</div>`,
    )
  if (p.deny_when !== undefined)
    clauses.push(
      `<div class="clause"><div class="clause-label danger">deny when</div>${renderExpression(p.deny_when)}</div>`,
    )
  const ann = renderAnnotations(p.annotations)
  const metaRows = [triple, ann ? kvRow('annotations', ann) : ''].filter(Boolean).join('')
  return `<article class="card" id="perm-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3></header>
    ${desc}
    <dl class="kv">${metaRows}</dl>
    ${clauses.join('\n')}
  </article>`
}

const renderPermissions = (perms: Record<string, Permission>): string => {
  const cards = Object.entries(perms)
    .map(([n, p]) => renderPermission(n, p))
    .join('\n')
  return wrapSection(
    'permissions',
    'Permissions',
    '誰が、何に対して、どんな条件で何ができるかを宣言する認可ルール。',
    `<div class="cards">${cards}</div>`,
    Object.keys(perms).length,
  )
}

// ─── section: objectives ───────────────────────────────────────────

const OBJ_KIND_ORDER = ['slo', 'lifetime', 'security', 'retention']

const renderObjective = (name: string, o: Objective): string => {
  const kind = o.kind ?? 'unknown'
  const desc = o.description ? `<p class="desc">${esc(o.description)}</p>` : ''
  const rows = Object.entries(o)
    .filter(([k]) => k !== 'kind' && k !== 'description')
    .map(([k, v]) =>
      kvRow(k.replace(/_/g, ' '), k === 'annotations' ? renderAnnotations(v) : renderValue(v)),
    )
    .join('')
  return `<article class="card" id="obj-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${badge(kind, `kind-${kind}`)}</header>
    ${desc}
    ${rows ? `<dl class="kv">${rows}</dl>` : ''}
  </article>`
}

const renderObjectives = (objs: Record<string, Objective>): string => {
  const grouped = new Map<string, Array<[string, Objective]>>()
  for (const [n, o] of Object.entries(objs)) {
    const k = o.kind ?? 'unknown'
    const bucket = grouped.get(k) ?? []
    bucket.push([n, o])
    grouped.set(k, bucket)
  }
  const ordered = [
    ...OBJ_KIND_ORDER.filter((k) => grouped.has(k)),
    ...[...grouped.keys()].filter((k) => !OBJ_KIND_ORDER.includes(k)),
  ]
  const groups = ordered
    .map((kind) => {
      const entries = grouped.get(kind) ?? []
      const items = entries.map(([n, o]) => renderObjective(n, o)).join('\n')
      return `<div class="group">
      <h3 class="grp-title">${esc(kind)} <span class="count">${entries.length}</span></h3>
      <div class="cards">${items}</div>
    </div>`
    })
    .join('\n')
  return wrapSection('objectives', 'Objectives', '', groups, Object.keys(objs).length)
}

// ─── section: assurance ────────────────────────────────────────────

const renderAssurance = (obligations: Record<string, AssuranceObligation>): string => {
  const cards = Object.entries(obligations)
    .map(([name, obligation]) => {
      const evidence = Object.entries(obligation.evidence ?? {})
        .map(([evidenceName, requirement]) => {
          const attributes = [
            requirement.kind ? badge(requirement.kind, `kind-${requirement.kind}`) : '',
            requirement.producer ? chip(`producer: ${requirement.producer}`, 'hint') : '',
            requirement.evaluation ? chip(`evaluation: ${requirement.evaluation}`, 'hint') : '',
            requirement.recheck ? chip(`recheck: ${requirement.recheck}`, 'hint') : '',
          ]
            .filter(Boolean)
            .join(' ')
          return `<article class="requirement">
            <header><code class="name">${esc(evidenceName)}</code>${attributes}</header>
            ${requirement.environments?.length ? `<div class="chip-row">${requirement.environments.map((environment) => chip(environment)).join(' ')}</div>` : ''}
            ${renderNamedReferences(requirement.covers)}
            ${requirement.procedure ? `<p><strong>procedure:</strong> <code>${esc(requirement.procedure)}</code></p>` : ''}
            ${requirement.oracle ? `<p><strong>oracle:</strong> ${esc(requirement.oracle)}</p>` : ''}
          </article>`
        })
        .join('')
      const approvalRequirement = obligation.approval
      const approval = approvalRequirement
        ? `<dl class="kv">
            ${approvalRequirement.role ? kvRow('approval role', chip(approvalRequirement.role)) : ''}
            ${approvalRequirement.when?.length ? kvRow('approval when', approvalRequirement.when.map((condition) => chip(condition)).join(' ')) : ''}
            ${approvalRequirement.decision_record !== undefined ? kvRow('decision record', badge(approvalRequirement.decision_record)) : ''}
          </dl>`
        : ''
      return `<article class="card" id="assurance-${esc(slug(name))}">
        <header><h3>${esc(name)}</h3>${obligation.risk_level ? badge(obligation.risk_level, `severity-${obligation.risk_level}`) : ''}</header>
        ${obligation.claim ? `<p class="desc"><strong>claim:</strong> ${esc(obligation.claim)}</p>` : ''}
        ${obligation.risk ? `<p><strong>risk:</strong> ${esc(obligation.risk)}</p>` : ''}
        ${renderNamedReferences(obligation.derived_from)}
        ${obligation.acceptance !== undefined ? `<div class="io"><div class="label">Acceptance</div>${renderValue(obligation.acceptance)}</div>` : ''}
        ${evidence ? `<div class="io"><div class="label">Evidence</div><div class="requirements">${evidence}</div></div>` : ''}
        ${approval}
        ${obligation.annotations ? `<dl class="kv">${kvRow('annotations', renderAnnotations(obligation.annotations))}</dl>` : ''}
      </article>`
    })
    .join('\n')
  return wrapSection(
    'assurance',
    'Assurance',
    '規範要件を満たしたと判定するための主張、リスク、合否基準、必要な検証。',
    `<div class="cards">${cards}</div>`,
    Object.keys(obligations).length,
  )
}

// ─── section wrapper + overview + TOC ──────────────────────────────

const wrapSection = (
  id: SectionKind,
  title: string,
  lead: string,
  body: string,
  count: number,
): string =>
  `<section id="${id}">
    <h2>${esc(title)} <span class="count">${count}</span></h2>
    ${lead ? `<p class="lead">${esc(lead)}</p>` : ''}
    ${body}
  </section>`

const SECTION_TITLES: Record<SectionKind, string> = {
  standards: 'Standards',
  vocabulary: 'Vocabulary',
  models: 'Models',
  interfaces: 'Interfaces',
  states: 'States',
  invariants: 'Invariants',
  scenarios: 'Scenarios',
  permissions: 'Permissions',
  objectives: 'Objectives',
  assurance: 'Assurance',
  user_experience: 'User Experience',
}

const renderOverview = (scl: SclDocument): string => {
  const present = SECTION_KINDS.filter((k) => scl[k] !== undefined)
  const stats = present
    .map((k) => {
      const section = scl[k]
      const n =
        k === 'user_experience'
          ? Object.keys(scl.user_experience?.screens ?? {}).length
          : Object.keys(section as Record<string, unknown>).length
      return `<a class="stat" href="#${k}"><span class="stat-num">${n}</span><span class="stat-label">${esc(SECTION_TITLES[k].toLowerCase())}</span></a>`
    })
    .join('')
  return `<section id="overview">
    <header class="page-header">
      <div class="eyebrow">Specification Core Language</div>
      <h1>${esc(scl.system)}</h1>
      <div class="page-meta">${badge(`spec ${scl.spec_version}`, 'version')}</div>
    </header>
    <div class="stats">${stats}</div>
  </section>`
}

const renderTOC = (scl: SclDocument): string => {
  const present = SECTION_KINDS.filter((k) => scl[k] !== undefined)
  const items: Array<readonly [string, string]> = [
    ['overview', 'Overview'],
    ...present.map((k) => [k, SECTION_TITLES[k]] as const),
  ]
  return `<nav class="toc" aria-label="Contents">
    <div class="toc-title">Contents</div>
    <ol>${items.map(([id, label]) => `<li><a href="#${id}">${esc(label)}</a></li>`).join('')}</ol>
  </nav>`
}

const renderOneSection = (k: SectionKind, scl: SclDocument): string => {
  switch (k) {
    case 'standards':
      return scl.standards ? renderStandards(scl.standards) : ''
    case 'vocabulary':
      return scl.vocabulary ? renderVocab(scl.vocabulary) : ''
    case 'models':
      return scl.models ? renderModels(scl.models) : ''
    case 'interfaces':
      return scl.interfaces ? renderInterfaces(scl.interfaces) : ''
    case 'states':
      return scl.states ? renderStates(scl.states) : ''
    case 'invariants':
      return scl.invariants ? renderInvariants(scl.invariants) : ''
    case 'scenarios':
      return scl.scenarios ? renderScenarios(scl.scenarios, buildXref(scl)) : ''
    case 'permissions':
      return scl.permissions ? renderPermissions(scl.permissions) : ''
    case 'objectives':
      return scl.objectives ? renderObjectives(scl.objectives) : ''
    case 'assurance':
      return scl.assurance ? renderAssurance(scl.assurance) : ''
    case 'user_experience':
      return scl.user_experience ? renderUserExperience(scl.user_experience) : ''
  }
}

// ─── CSS ───────────────────────────────────────────────────────────

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

/* layout */
.layout {
  display: grid; grid-template-columns: 240px minmax(0,1fr); gap: 40px;
  max-width: 1280px; margin: 0 auto; padding: 32px 24px 96px;
}
@media (max-width: 900px) {
  .layout { grid-template-columns: 1fr; padding: 16px; gap: 24px; }
  .toc { position: static !important; max-height: none !important; }
}
.toc {
  position: sticky; top: 24px; align-self: start;
  max-height: calc(100vh - 48px); overflow-y: auto; font-size: 14px;
}
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
section { margin-bottom: 56px; scroll-margin-top: 24px; }

/* overview */
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
.badge-version {
  background: var(--surface-2); color: var(--fg-soft); border-color: var(--border);
}
.badge-context { background: var(--warn-soft); color: var(--warn); }
.badge-kind-entity { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-kind-value_object {
  background: var(--surface-2); color: var(--fg-soft); border: 1px solid var(--border);
}
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
.badge-method-cli {
  background: var(--surface-2); color: var(--fg-soft); border-color: var(--border);
}
.badge-severity-must { background: var(--danger-soft); color: var(--danger); }
.badge-severity-should { background: var(--warn-soft); color: var(--warn); }
.badge-adoption-required { background: var(--ok-soft); color: var(--ok); }
.badge-adoption-optional { background: var(--warn-soft); color: var(--warn); }
.badge-adoption-excluded { background: var(--danger-soft); color: var(--danger); }
.badge-strength, .badge-category {
  background: var(--surface-2); color: var(--fg-soft); border-color: var(--border);
}

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
.chip-iface-ref {
  background: rgba(91,91,230,.14); color: var(--accent); border-color: transparent;
}
.chip-initial {
  background: rgba(91,91,230,.14); color: var(--accent); border-color: transparent; font-weight: 600;
}
.chip-terminal { background: var(--warn-soft); color: var(--warn); border-color: transparent; }
.chip-tag { background: var(--surface-2); color: var(--fg-soft); }
.chip-hint { background: var(--surface-2); color: var(--muted); }

.name { color: var(--fg); font-weight: 600; }
.type { color: var(--fg-soft); }
.path {
  background: var(--surface-2); border: 1px solid var(--border);
  padding: 2px 8px; border-radius: 6px; font-size: 13px;
}
.muted { color: var(--muted); }
.text { color: var(--fg); }

/* kv list */
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

/* fields tables */
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
.vocab-entry:target {
  border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft);
}
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

/* scenarios: natural-language steps */
.iface-step { margin: 6px 0 2px; }
.step-tpl { color: var(--fg-soft); }
.scn-steps { margin: 8px 0 0; padding-left: 26px; }
.scn-step { padding: 3px 0; line-height: 1.7; }
.scn-step::marker { color: var(--muted); font-size: 0.85em; }
.step-arg {
  background: var(--surface-2); color: var(--fg); border: 1px solid var(--border);
  padding: 0 5px; border-radius: 4px;
}
.chip-ref { background: var(--accent-soft); color: var(--accent); border-color: transparent; }

/* standards and user experience */
.requirements { display: grid; gap: 10px; margin-top: 12px; }
.requirement {
  border: 1px solid var(--border); border-radius: 9px; padding: 12px 14px;
  background: var(--surface-2);
}
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
.clause {
  margin: 10px 0; padding: 10px 12px; background: var(--surface-2); border-radius: 8px;
}
.clause-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase;
  color: var(--muted); margin-bottom: 6px;
}
.clause-label.ok { color: var(--ok); }
.clause-label.danger { color: var(--danger); }
.expr {
  display: inline-block; padding: 2px 8px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 4px;
  font-size: 13px;
}
.expr-list { margin: 0; padding-left: 18px; }
.expr-list li { margin: 4px 0; }
.expr-op { margin: 6px 0 6px 12px; }
.expr-op-label {
  font-size: 11px; font-weight: 700; letter-spacing: 0.08em; color: var(--accent);
  margin-bottom: 4px;
}
.expr-quant {
  display: flex; flex-wrap: wrap; align-items: center; gap: 6px;
  margin: 4px 0 4px 12px; padding: 4px 8px;
  background: var(--surface); border: 1px solid var(--border); border-radius: 6px;
}
.expr-quant-sym { color: var(--accent); font-weight: 700; font-size: 14px; }

/* property clauses (extension: assuming / eventually) */
.clause-assuming { background: var(--surface); border: 1px solid var(--border); }
.clause-eventually { background: var(--accent-soft); }
.clause-label.accent { color: var(--accent); }
.clause-label.muted { color: var(--muted); }
.within { font-size: 11px; font-weight: 500; color: var(--muted); text-transform: none; letter-spacing: 0; margin-left: 6px; }

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
.badge-direction-produce { background: var(--accent-soft); color: var(--accent); }
.badge-direction-consume { background: var(--ok-soft); color: var(--ok); }
.badge-gql-query { background: var(--ok-soft); color: var(--ok); }
.badge-gql-mutation { background: rgba(91,91,230,.14); color: var(--accent); }
.badge-gql-subscription { background: var(--warn-soft); color: var(--warn); }
.sub-label { font-size: 11px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; color: var(--muted); margin: 6px 0 2px; }

/* interface step label (reused by scenarios cross-refs) */
.step-kind-label {
  font-size: 10px; font-weight: 700; letter-spacing: 0.1em;
  padding: 2px 6px; border-radius: 4px;
  background: var(--surface-2); color: var(--muted);
}
`

// Scrollspy: highlight the TOC link whose section currently dominates
// the viewport. Static script, no SCL data injected.
const SCROLLSPY = `
(() => {
  const sections = Array.from(document.querySelectorAll('main section[id]'));
  if (!sections.length) return;
  const links = new Map();
  for (const a of document.querySelectorAll('.toc a')) {
    const href = a.getAttribute('href');
    if (href && href.startsWith('#')) links.set(href.slice(1), a);
  }
  const setActive = (id) => {
    for (const a of links.values()) a.classList.remove('active');
    const a = links.get(id);
    if (a) a.classList.add('active');
  };
  const TRIGGER = 120;
  let raf = 0;
  const update = () => {
    raf = 0;
    let current = sections[0];
    for (const s of sections) {
      if (s.getBoundingClientRect().top - TRIGGER <= 0) current = s;
      else break;
    }
    setActive(current.id);
  };
  const onScroll = () => { if (!raf) raf = requestAnimationFrame(update); };
  addEventListener('scroll', onScroll, { passive: true });
  addEventListener('resize', onScroll, { passive: true });
  update();
})();
`

export const render = (scl: SclDocument): string => {
  const sections = SECTION_KINDS.map((k) => renderOneSection(k, scl))
    .filter(Boolean)
    .join('\n')
  const html = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(scl.system)} — SCL</title>
<style>${CSS}</style>
</head>
<body>
<div class="layout">
${renderTOC(scl)}
<main>
${renderOverview(scl)}
${sections}
</main>
</div>
<script>${SCROLLSPY}</script>
</body>
</html>
`
  return html.replace(/[ \t]+$/gm, '')
}
