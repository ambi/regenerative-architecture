/**
 * SCL renderer — one HTML fragment per SCL section, plus the SCL-tab
 * shell (overview banner + per-section sidebar TOC).
 *
 * Derived from spec/scl.yaml (Render interface). No I/O, deterministic.
 */

import {
  badge,
  chip,
  esc,
  isObj,
  kvRow,
  link,
  renderAnnotations,
  renderConstraints,
  renderValue,
  slug,
  typeText,
} from './html.ts'
import {
  type Binding,
  type ContextMapEntry,
  type Field,
  type GlossaryEntry,
  type Interface,
  type Invariant,
  type Model,
  type Objective,
  type Permission,
  type Scenario,
  SECTION_KINDS,
  type SclBundle,
  type SclContextDocument,
  type SclDocument,
  type SectionKind,
  type Standard,
  type StateMachine,
  type UserExperience,
} from './types.ts'

// ─── cross-section references ──────────────────────────────────────

const referenceAnchor = (section: string, name: string): string | undefined => {
  const prefixes: Record<string, string> = {
    glossary: 'glossary',
    models: 'model',
    events: 'model',
    interfaces: 'iface',
    states: 'state',
    invariants: 'inv',
    scenarios: 'scn',
    permissions: 'perm',
    objectives: 'obj',
    standards: 'std',
    context_map: 'ctx',
  }
  const prefix = prefixes[section]
  return prefix ? `#${prefix}-${slug(name)}` : undefined
}

const prefixAnchors = (html: string, prefix: string): string => {
  const knownIds = new Set([...SECTION_KINDS, 'scl-overview'])
  const idRe = new RegExp(
    `^(${[
      'std',
      'req',
      'ctx',
      'glossary',
      'model',
      'iface',
      'state',
      'inv',
      'scn',
      'perm',
      'obj',
      'screen',
      'ux',
      'diagram',
    ].join('|')})-`,
  )
  const shouldPrefix = (id: string): boolean => knownIds.has(id) || idRe.test(id)
  return html
    .replace(/\bid="([^"]+)"/g, (_m, id: string) =>
      shouldPrefix(id) ? `id="${esc(`${prefix}-${id}`)}"` : `id="${esc(id)}"`,
    )
    .replace(/\bhref="#([^"]+)"/g, (_m, id: string) =>
      shouldPrefix(id) ? `href="#${esc(`${prefix}-${id}`)}"` : `href="#${esc(id)}"`,
    )
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

// ─── derived diagrams ──────────────────────────────────────────────

interface DiagramNode {
  id: string
  label: string
  href?: string
  kind?: string
}

interface DiagramEdge {
  from: string
  to: string
  label?: string
  href?: string
}

const renderDiagram = (
  id: string,
  title: string,
  description: string,
  nodes: DiagramNode[],
  edges: DiagramEdge[],
): string => {
  if (!nodes.length) return ''
  const nodeW = 184
  const nodeH = 58
  const gapX = 72
  const gapY = 68
  const pad = 36
  const cols = Math.max(1, Math.ceil(Math.sqrt(nodes.length)))
  const positions = new Map<string, { x: number; y: number; cx: number; cy: number }>()
  nodes.forEach((node, index) => {
    const col = index % cols
    const row = Math.floor(index / cols)
    const x = pad + col * (nodeW + gapX)
    const y = pad + row * (nodeH + gapY)
    positions.set(node.id, { x, y, cx: x + nodeW / 2, cy: y + nodeH / 2 })
  })
  const rows = Math.ceil(nodes.length / cols)
  const width = pad * 2 + cols * nodeW + Math.max(0, cols - 1) * gapX
  const height = pad * 2 + rows * nodeH + Math.max(0, rows - 1) * gapY
  const viewBox = `0 0 ${width} ${height}`
  const edgeSvg = edges
    .map((edge, index) => {
      const from = positions.get(edge.from)
      const to = positions.get(edge.to)
      if (!from || !to) return ''
      const dx = to.cx - from.cx
      const dy = to.cy - from.cy
      const len = Math.max(1, Math.hypot(dx, dy))
      const sx = from.cx + (dx / len) * (nodeW / 2)
      const sy = from.cy + (dy / len) * (nodeH / 2)
      const tx = to.cx - (dx / len) * (nodeW / 2)
      const ty = to.cy - (dy / len) * (nodeH / 2)
      const labelX = (sx + tx) / 2
      const labelY = (sy + ty) / 2 - 6
      const line = `<g class="diagram-edge" id="diagram-${esc(id)}-edge-${index}">
        <line x1="${sx.toFixed(1)}" y1="${sy.toFixed(1)}" x2="${tx.toFixed(1)}" y2="${ty.toFixed(1)}" marker-end="url(#arrow-${esc(id)})"></line>
        ${edge.label ? `<text x="${labelX.toFixed(1)}" y="${labelY.toFixed(1)}">${esc(edge.label)}</text>` : ''}
      </g>`
      return edge.href ? `<a href="${esc(edge.href)}">${line}</a>` : line
    })
    .join('')
  const nodeSvg = nodes
    .map((node) => {
      const pos = positions.get(node.id)
      if (!pos) return ''
      const body = `<g class="diagram-node diagram-node-${esc(node.kind ?? 'default')}" id="diagram-${esc(id)}-node-${esc(slug(node.id))}">
        <rect x="${pos.x}" y="${pos.y}" width="${nodeW}" height="${nodeH}" rx="8"></rect>
        <text x="${pos.cx}" y="${pos.cy + 5}">${esc(node.label)}</text>
      </g>`
      return node.href ? `<a href="${esc(node.href)}">${body}</a>` : body
    })
    .join('')
  return `<div class="diagram-card" id="diagram-${esc(id)}">
    <div class="diagram-head">
      <div>
        <div class="label">Diagram</div>
        <h3>${esc(title)}</h3>
        ${description ? `<p class="desc">${esc(description)}</p>` : ''}
      </div>
      <div class="diagram-tools" aria-label="${esc(title)} controls">
        <button type="button" data-diagram-zoom="in" title="Zoom in">+</button>
        <button type="button" data-diagram-zoom="out" title="Zoom out">-</button>
        <button type="button" data-diagram-fit title="Fit to view">Fit</button>
      </div>
    </div>
    <div class="diagram-viewport" data-diagram>
      <svg viewBox="${viewBox}" data-diagram-svg data-diagram-viewbox="${viewBox}" role="img" aria-label="${esc(title)}">
        <defs>
          <marker id="arrow-${esc(id)}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z"></path>
          </marker>
        </defs>
        ${edgeSvg}
        ${nodeSvg}
      </svg>
    </div>
  </div>`
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
    <td>${
      f.fields
        ? `<div class="label">inline</div>${fieldsTable(f.fields)}`
        : `<code class="type">${esc(typeText(f.type))}</code>`
    }</td>
    <td>${f.fields ? '' : f.optional ? badge('optional', 'optional') : badge('required', 'required')}</td>
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

const renderExprOp = (op: string, operand: unknown): string => {
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
  if ((op === 'exists' || op === 'not_exists') && typeof operand === 'string') {
    const label = op === 'exists' ? 'EXISTS' : 'NOT EXISTS'
    return `<code class="expr">${label}(${esc(operand)})</code>`
  }
  if ((op === 'count' || op === 'len') && (typeof operand === 'string' || isObj(operand))) {
    const inner = typeof operand === 'string' ? esc(operand) : renderExpression(operand)
    return `<code class="expr">${op.toUpperCase()}(${inner})</code>`
  }
  return `<div class="expr-op">
    <div class="expr-op-label">${esc(op.toUpperCase())}</div>
    ${renderExpression(operand)}
  </div>`
}

// ─── section: standards ────────────────────────────────────────────

const renderStandards = (standards: Record<string, Standard>): string => {
  const cards = Object.entries(standards)
    .map(([name, standard]) => {
      const requirements = (standard.requirements ?? [])
        .map((req) => {
          const adoption = req.adoption ?? 'required'
          return `<article class="requirement" id="req-${esc(slug(req.id ?? ''))}">
            <header>
              <code class="name">${esc(req.id)}</code>
              ${badge(adoption, `adoption-${adoption}`)}
              ${req.strength ? badge(req.strength, 'strength') : ''}
              ${req.section ? chip(req.section, 'hint') : ''}
            </header>
            ${req.statement ? `<p>${esc(req.statement)}</p>` : ''}
            ${req.reason ? `<p class="exclusion-reason"><strong>reason:</strong> ${esc(req.reason)}</p>` : ''}
            ${renderNamedReferences(req.relates_to)}
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

// ─── section: context_map ──────────────────────────────────────────

const renderContextMapEntry = (name: string, c: ContextMapEntry): string => {
  const desc = c.description ? `<p class="desc">${esc(c.description)}</p>` : ''
  const publishes = c.publishes?.length
    ? `<div class="reference-row"><span class="reference-label">publishes</span>${c.publishes
        .map((item) => chip(item, 'ref'))
        .join(' ')}</div>`
    : ''
  const deps = Object.entries(c.depends_on ?? {})
    .map(([depName, dep]) => {
      const ref = link(`#ctx-${slug(depName)}`, depName, 'ref')
      const uses = dep.uses?.length ? ` ${dep.uses.map((item) => chip(item)).join(' ')}` : ''
      const via = dep.via ? badge(dep.via, 'hint') : ''
      return `<li>${ref} ${via}${uses}${dep.reason ? ` <span class="muted">— ${esc(dep.reason)}</span>` : ''}</li>`
    })
    .join('')
  const depsBlock = deps
    ? `<div class="io"><div class="label">Depends on</div><ul class="vlist">${deps}</ul></div>`
    : ''
  const ann = renderAnnotations(c.annotations)
  return `<article class="card" id="ctx-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${c.path ? chip(c.path, 'hint') : ''}</header>
    ${desc}
    ${publishes}
    ${depsBlock}
    ${ann ? `<dl class="kv">${kvRow('annotations', ann)}</dl>` : ''}
  </article>`
}

const renderContextMap = (contextMap: Record<string, ContextMapEntry>): string => {
  const cards = Object.entries(contextMap)
    .map(([n, c]) => renderContextMapEntry(n, c))
    .join('\n')
  const nodes = Object.keys(contextMap).map((name) => ({
    id: name,
    label: name,
    href: `#ctx-${slug(name)}`,
    kind: 'context',
  }))
  const edges = Object.entries(contextMap).flatMap(([name, entry]) =>
    Object.entries(entry.depends_on ?? {}).map(([depName, dep]) => ({
      from: name,
      to: depName,
      label: dep.via ?? 'depends on',
      href: `#ctx-${slug(depName)}`,
    })),
  )
  const diagram = renderDiagram(
    'context-map',
    'Context dependencies',
    'Context node と dependency edge は context_map から派生する。',
    nodes,
    edges,
  )
  return wrapSection(
    'context_map',
    'Context Map',
    'Bounded Context 間の公開言語と依存方向。',
    `${diagram}<div class="cards">${cards}</div>`,
    Object.keys(contextMap).length,
  )
}

// ─── section: glossary ─────────────────────────────────────────────

const renderGlossary = (entries: Record<string, GlossaryEntry>): string => {
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
      return `<article class="vocab-entry" id="glossary-${esc(slug(name))}">
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
    'glossary',
    'Glossary',
    '曖昧語、別名、翻訳、外部標準語を説明する補助用語集。',
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
  const get = (k: string): unknown => (b as Record<string, unknown>)[k]
  const str = (k: string): string | undefined => {
    const v = get(k)
    return typeof v === 'string' ? v : undefined
  }
  switch (b.kind) {
    case 'http': {
      const head: string[] = []
      const method = str('method')
      const path = str('path')
      const form = str('request_form')
      const status = get('successful_status_codes')
      if (method) head.push(badge(method, `method-${method.toLowerCase()}`))
      if (path) head.push(`<code class="path">${esc(path)}</code>`)
      if (Array.isArray(status) && status.length)
        head.push(chip(`response: ${status.join(', ')}`, 'hint'))
      if (form) head.push(chip(`form: ${form}`, 'hint'))
      const headers = get('headers')
      return `<div class="binding-head">${head.join(' ')}</div>${
        isObj(headers) ? ioTable(headers as Record<string, Field>, 'HTTP Headers') : ''
      }`
    }
    case 'grpc': {
      const head: string[] = []
      const service = str('service')
      const method = str('method')
      const streaming = str('streaming')
      if (service && method) head.push(`<code class="path">${esc(service)}.${esc(method)}</code>`)
      else if (method) head.push(`<code class="path">${esc(method)}</code>`)
      if (streaming) head.push(chip(`streaming: ${streaming}`, 'hint'))
      return `<div class="binding-head">${head.join(' ')}</div>`
    }
    case 'cli': {
      const head: string[] = []
      const command = str('command')
      if (command) head.push(`<code class="path">${esc(command)}</code>`)
      const out: string[] = [`<div class="binding-head">${head.join(' ')}</div>`]
      const args = get('args')
      if (Array.isArray(args) && args.length) {
        const rows = args
          .map((a) => {
            const x = (a ?? {}) as Record<string, unknown>
            return `<tr>
              <td><code class="name">${esc(String(x.name ?? ''))}</code></td>
              <td>${x.position !== undefined ? chip(`pos ${x.position}`, 'hint') : ''}</td>
              <td>${x.required ? badge('required', 'required') : badge('optional', 'optional')}</td>
              <td>${x.repeatable ? chip('repeatable', 'hint') : ''}</td>
            </tr>`
          })
          .join('')
        out.push(
          `<div class="io"><div class="label">Args (positional)</div><table class="fields"><thead><tr><th>Name</th><th>Position</th><th>Presence</th><th>Note</th></tr></thead><tbody>${rows}</tbody></table></div>`,
        )
      }
      const flags = get('flags')
      if (Array.isArray(flags) && flags.length) {
        const rows = flags
          .map((f) => {
            const x = (f ?? {}) as Record<string, unknown>
            return `<tr>
              <td><code class="name">--${esc(String(x.name ?? ''))}</code></td>
              <td>${x.short ? `<code>-${esc(String(x.short))}</code>` : ''}</td>
              <td>${x.required ? badge('required', 'required') : badge('optional', 'optional')}</td>
              <td>${x.repeatable ? chip('repeatable', 'hint') : ''}</td>
            </tr>`
          })
          .join('')
        out.push(
          `<div class="io"><div class="label">Flags</div><table class="fields"><thead><tr><th>Long</th><th>Short</th><th>Presence</th><th>Note</th></tr></thead><tbody>${rows}</tbody></table></div>`,
        )
      }
      const meta: string[] = []
      const stdin = get('stdin')
      const stdout = get('stdout')
      const exitCodes = get('exit_codes')
      if (stdin !== undefined) meta.push(kvRow('stdin', `<code>${esc(typeText(stdin))}</code>`))
      if (stdout !== undefined) meta.push(kvRow('stdout', `<code>${esc(typeText(stdout))}</code>`))
      if (isObj(exitCodes) && Object.keys(exitCodes).length) {
        const codes = Object.entries(exitCodes)
          .map(([k, v]) => `${esc(k)}: <code>${esc(String(v))}</code>`)
          .join(' · ')
        meta.push(kvRow('exit codes', codes))
      }
      if (meta.length) out.push(`<dl class="kv">${meta.join('')}</dl>`)
      return out.join('')
    }
    case 'event': {
      const head: string[] = []
      const channel = str('channel')
      const direction = str('direction')
      const delivery = str('delivery')
      const ordering = str('ordering')
      const pk = str('partition_key')
      if (channel) head.push(`<code class="path">${esc(channel)}</code>`)
      if (direction) head.push(badge(direction, `direction-${direction}`))
      if (delivery) head.push(chip(`delivery: ${delivery}`, 'hint'))
      if (ordering) head.push(chip(`ordering: ${ordering}`, 'hint'))
      const meta = pk
        ? `<dl class="kv">${kvRow('partition key', `<code>${esc(pk)}</code>`)}</dl>`
        : ''
      return `<div class="binding-head">${head.join(' ')}</div>${meta}`
    }
    case 'graphql': {
      const head: string[] = []
      const op = str('operation')
      const field = str('field')
      if (op) head.push(badge(op, `gql-${op}`))
      if (field) head.push(`<code class="path">${esc(field)}</code>`)
      return `<div class="binding-head">${head.join(' ')}</div>`
    }
    case 'sdk': {
      const fn = str('function')
      return fn ? `<div class="binding-head"><code class="path">${esc(fn)}</code></div>` : ''
    }
    case 'schedule': {
      const head: string[] = []
      const cron = str('cron')
      const every = str('every')
      if (cron) head.push(`<code class="path">cron: ${esc(cron)}</code>`)
      if (every) head.push(`<code class="path">every: ${esc(every)}</code>`)
      return `<div class="binding-head">${head.join(' ')}</div>`
    }
    default: {
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
  const desc = b.description ? `<p class="desc">${esc(b.description)}</p>` : ''
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
    '外部との契約。入力・出力・エラー・発行イベントと、それを露出する複数のトランスポート (bindings)。',
    `<div class="cards">${cards}</div>`,
    Object.keys(ifaces).length,
  )
}

// ─── section: states ───────────────────────────────────────────────

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
  const diagrams = Object.entries(sms)
    .map(([name, sm]) => {
      const states = new Set<string>()
      for (const transition of sm.transitions ?? []) {
        if (transition.from) states.add(transition.from)
        if (transition.to) states.add(transition.to)
      }
      if (sm.initial) states.add(sm.initial)
      for (const state of sm.terminal ?? []) states.add(state)
      const terminal = new Set(sm.terminal ?? [])
      const nodes = [...states].map((state) => ({
        id: state,
        label: state,
        href: `#state-${slug(name)}`,
        kind: state === sm.initial ? 'initial' : terminal.has(state) ? 'terminal' : 'state',
      }))
      const edges = (sm.transitions ?? [])
        .filter((transition) => transition.from && transition.to)
        .map((transition) => ({
          from: String(transition.from),
          to: String(transition.to),
          label: String(transition.event ?? transition.on ?? ''),
          href: `#state-${slug(name)}`,
        }))
      return renderDiagram(
        `state-${slug(name)}`,
        `${name} transitions`,
        'State node と transition edge は states.transitions から派生する。',
        nodes,
        edges,
      )
    })
    .join('\n')
  return wrapSection(
    'states',
    'States',
    '',
    `${diagrams}<div class="cards">${cards}</div>`,
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

const buildXref = (scl: SclDocument): Map<string, string> => {
  const idx = new Map<string, string>()
  for (const n of Object.keys(scl.interfaces ?? {})) idx.set(n, `#iface-${slug(n)}`)
  for (const n of Object.keys(scl.models ?? {})) idx.set(n, `#model-${slug(n)}`)
  return idx
}

const renderStepText = (raw: string, xref: Map<string, string>): string =>
  esc(raw).replace(/&quot;(.*?)&quot;/g, (_m, inner: string) => {
    const href = xref.get(inner)
    return href
      ? `<a class="chip chip-ref" href="${esc(href)}">${inner}</a>`
      : `<code class="step-arg">${inner}</code>`
  })

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
  const metaRows = [
    s.goal ? kvRow('goal', esc(s.goal)) : '',
    s.primary_actor ? kvRow('primary actor', chip(s.primary_actor)) : '',
    s.scope ? kvRow('scope', chip(s.scope)) : '',
    s.level ? kvRow('level', chip(s.level)) : '',
  ]
    .filter(Boolean)
    .join('')
  const listBlock = (label: string, items?: string[]) =>
    items?.length
      ? `<div class="sub"><div class="label">${esc(label)}</div><ul class="vlist">${items
          .map((item) => `<li>${renderStepText(item, xref)}</li>`)
          .join('')}</ul></div>`
      : ''
  const steps = (s.steps ?? [])
    .map((st) => `<li class="scn-step">${renderStepText(String(st), xref)}</li>`)
    .join('')
  const mainSuccess = (s.main_success ?? [])
    .map((st) => `<li class="scn-step">${renderStepText(String(st), xref)}</li>`)
    .join('')
  const extensions = (s.extensions ?? [])
    .map(
      (ext) => `<article class="requirement">
        <header>${ext.at !== undefined ? chip(`at ${ext.at}`, 'hint') : ''}<code class="name">${esc(ext.condition ?? '')}</code></header>
        <ol class="scn-steps">${(ext.steps ?? [])
          .map((st) => `<li class="scn-step">${renderStepText(String(st), xref)}</li>`)
          .join('')}</ol>
      </article>`,
    )
    .join('')
  const where = s.where?.length ? renderWhere(s.where) : ''
  return `<article class="card scenario" id="scn-${esc(slug(name))}">
    <header><h3>${esc(name)}</h3>${tags}</header>
    ${desc}
    ${metaRows ? `<dl class="kv">${metaRows}</dl>` : ''}
    ${ann ? `<dl class="kv">${kvRow('annotations', ann)}</dl>` : ''}
    ${listBlock('Preconditions', s.preconditions)}
    ${listBlock('Success guarantees', s.success_guarantees)}
    ${mainSuccess ? `<div class="sub"><div class="label">Main success</div><ol class="scn-steps">${mainSuccess}</ol></div>` : ''}
    ${steps ? `<div class="sub"><div class="label">Steps</div><ol class="scn-steps">${steps}</ol></div>` : ''}
    ${extensions ? `<div class="sub"><div class="label">Extensions</div><div class="requirements">${extensions}</div></div>` : ''}
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
      ['operation', p.operation],
      ['resource', p.resource],
    ] as const
  )
    .map(([k, v]) =>
      kvRow(k, v ? `<code class="name">${esc(v)}</code>` : '<span class="muted">—</span>'),
    )
    .join('')
  const clauses: string[] = []
  if (p.protects?.length) {
    clauses.push(
      `<div class="reference-row"><span class="reference-label">protects</span>${p.protects
        .map((target) => {
          const [section, name] = target.split('.', 2)
          const href = section && name ? referenceAnchor(section, name) : undefined
          return href ? link(href, target, 'ref') : chip(target)
        })
        .join(' ')}</div>`,
    )
  }
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

// ─── section: user_experience ──────────────────────────────────────

const renderUserExperience = (ux: UserExperience): string => {
  const screenNodes = Object.keys(ux.screens ?? {}).map((name) => ({
    id: name,
    label: name,
    href: `#screen-${slug(name)}`,
    kind: 'screen',
  }))
  const externalNodes = (ux.transitions ?? []).some(
    (transition) => transition.external && !transition.to,
  )
    ? [{ id: '__external__', label: 'External', kind: 'external' }]
    : []
  const uxDiagram = renderDiagram(
    'ux-transitions',
    'Screen transitions',
    'Screen node と transition edge は user_experience.screens / transitions から派生する。',
    [...screenNodes, ...externalNodes],
    (ux.transitions ?? [])
      .filter((transition) => transition.from && (transition.to || transition.external))
      .map((transition) => ({
        from: String(transition.from),
        to: transition.to ? String(transition.to) : '__external__',
        label: String(transition.trigger ?? transition.interface ?? ''),
        href: transition.to ? `#screen-${slug(transition.to)}` : undefined,
      })),
  )
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
      (t) => `<tr>
        <td>${t.from ? link(`#screen-${slug(t.from)}`, t.from) : '<span class="muted">system</span>'}</td>
        <td><code>${esc(t.trigger)}</code></td>
        <td>${
          t.to
            ? link(`#screen-${slug(t.to)}`, t.to)
            : t.external
              ? badge('external', 'optional')
              : ''
        }</td>
        <td>${t.interface ? link(`#iface-${slug(t.interface)}`, t.interface, 'iface-ref') : ''}</td>
      </tr>`,
    )
    .join('')

  const requirements = (ux.requirements ?? [])
    .map((req) => {
      const refs: Record<string, string[]> = {}
      if (req.interfaces) refs.interfaces = req.interfaces
      if (req.standards) refs.standards = req.standards
      if (req.scenarios) refs.scenarios = req.scenarios
      if (req.invariants) refs.invariants = req.invariants
      return `<article class="requirement" id="ux-${esc(slug(req.id ?? ''))}">
        <header>
          <code class="name">${esc(req.id)}</code>
          ${badge(req.adoption ?? 'required', `adoption-${req.adoption ?? 'required'}`)}
          ${req.category ? badge(req.category, 'category') : ''}
        </header>
        ${req.statement ? `<p>${esc(req.statement)}</p>` : ''}
        ${
          req.screens?.length
            ? `<div class="reference-row"><span class="reference-label">screens</span>${req.screens
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
    ${uxDiagram}
    <div class="group"><h3 class="grp-title">Screens <span class="count">${Object.keys(ux.screens ?? {}).length}</span></h3><div class="cards">${screens}</div></div>
    <div class="group"><h3 class="grp-title">Transitions <span class="count">${ux.transitions?.length ?? 0}</span></h3><table class="fields"><thead><tr><th>From</th><th>Trigger</th><th>To</th><th>Interface</th></tr></thead><tbody>${transitions}</tbody></table></div>
    <div class="group"><h3 class="grp-title">Requirements <span class="count">${ux.requirements?.length ?? 0}</span></h3><div class="requirements">${requirements}</div></div>`,
    Object.keys(ux.screens ?? {}).length,
  )
}

// ─── section wrapper + dispatcher ──────────────────────────────────

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

export const SECTION_TITLES: Record<SectionKind, string> = {
  standards: 'Standards',
  context_map: 'Context Map',
  glossary: 'Glossary',
  models: 'Models',
  interfaces: 'Interfaces',
  states: 'States',
  invariants: 'Invariants',
  scenarios: 'Scenarios',
  permissions: 'Permissions',
  objectives: 'Objectives',
  user_experience: 'User Experience',
}

const renderOneSection = (k: SectionKind, scl: SclDocument): string => {
  switch (k) {
    case 'standards':
      return scl.standards ? renderStandards(scl.standards) : ''
    case 'context_map':
      return scl.context_map ? renderContextMap(scl.context_map) : ''
    case 'glossary':
      return scl.glossary ? renderGlossary(scl.glossary) : ''
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
    case 'user_experience':
      return scl.user_experience ? renderUserExperience(scl.user_experience) : ''
  }
}

export const sclSectionsPresent = (scl: SclDocument): SectionKind[] =>
  SECTION_KINDS.filter((k) => scl[k] !== undefined)

const isSclBundle = (scl: SclDocument | SclBundle): scl is SclBundle =>
  'root' in scl && 'contexts' in scl

const renderSingleSclTab = (scl: SclDocument): string => {
  const sections = sclSectionsPresent(scl)
    .map((k) => renderOneSection(k, scl))
    .filter(Boolean)
    .join('\n')
  const stats = sclSectionsPresent(scl)
    .map((k) => {
      const section = scl[k]
      const n =
        k === 'user_experience'
          ? Object.keys(scl.user_experience?.screens ?? {}).length
          : Object.keys(section as Record<string, unknown>).length
      return `<a class="stat" href="#${k}"><span class="stat-num">${n}</span><span class="stat-label">${esc(SECTION_TITLES[k].toLowerCase())}</span></a>`
    })
    .join('')
  return `<section id="scl-overview" class="tab-overview">
    <header class="page-header">
      <div class="eyebrow">Specification Core Language · spec ${esc(scl.spec_version)}</div>
      <h1>${esc(scl.system)}</h1>
    </header>
    <div class="stats">${stats}</div>
  </section>
  ${sections}`
}

const renderContextDocument = (ctx: SclContextDocument): string => {
  const contextName = ctx.document.context ?? ctx.name
  const prefix = `context-${slug(contextName)}`
  const sections = sclSectionsPresent(ctx.document)
    .filter((k) => k !== 'context_map')
    .map((k) => prefixAnchors(renderOneSection(k, ctx.document), prefix))
    .filter(Boolean)
    .join('\n')
  const stats = sclSectionsPresent(ctx.document)
    .filter((k) => k !== 'context_map')
    .map((k) => {
      const section = ctx.document[k]
      const n =
        k === 'user_experience'
          ? Object.keys(ctx.document.user_experience?.screens ?? {}).length
          : Object.keys(section as Record<string, unknown>).length
      return `<a class="stat" href="#${esc(`${prefix}-${k}`)}"><span class="stat-num">${n}</span><span class="stat-label">${esc(SECTION_TITLES[k].toLowerCase())}</span></a>`
    })
    .join('')
  return `<section id="${esc(prefix)}" class="context-document">
    <header class="page-header">
      <div class="eyebrow">Bounded Context · ${esc(ctx.path)}</div>
      <h1>${esc(contextName)}</h1>
    </header>
    <div class="stats">${stats}</div>
  </section>
  ${sections}`
}

export const renderSclTab = (input: SclDocument | SclBundle): string => {
  if (!isSclBundle(input)) return renderSingleSclTab(input)
  const root = `<div class="scl-context-pane active" data-scl-context-pane="overview">
    ${renderSingleSclTab(input.root)}
  </div>`
  const contextLinks = input.contexts
    .map((ctx) => {
      const contextName = ctx.document.context ?? ctx.name
      const prefix = `context-${slug(contextName)}`
      return `<a class="context-tab-link" data-scl-context-link="${esc(prefix)}" href="${esc(`#tab=scl&sec=${prefix}`)}">${esc(contextName)}</a>`
    })
    .join('')
  const contextNav = `<nav class="context-tab-bar" aria-label="SCL contexts">
    <a class="context-tab-link active" data-scl-context-link="overview" href="${esc('#tab=scl&sec=scl-overview')}">Overview</a>
    ${contextLinks}
  </nav>`
  const contexts = input.contexts
    .map((ctx) => {
      const contextName = ctx.document.context ?? ctx.name
      const prefix = `context-${slug(contextName)}`
      return `<div class="scl-context-pane" data-scl-context-pane="${esc(prefix)}">
        ${renderContextDocument(ctx)}
      </div>`
    })
    .join('\n')
  return `${contextNav}\n${root}${contexts ? `\n${contexts}` : ''}`
}

export interface SclTocItem {
  id: string
  label: string
  sclContext?: string
}

const singleSclTocItems = (scl: SclDocument, sclContext?: string): SclTocItem[] => [
  { id: 'scl-overview', label: 'Overview', sclContext },
  ...sclSectionsPresent(scl).map((k) => ({ id: k, label: SECTION_TITLES[k], sclContext })),
]

const contextTocItems = (ctx: SclContextDocument): SclTocItem[] => {
  const contextName = ctx.document.context ?? ctx.name
  const prefix = `context-${slug(contextName)}`
  return [
    { id: prefix, label: 'Overview', sclContext: prefix },
    ...sclSectionsPresent(ctx.document)
      .filter((k) => k !== 'context_map')
      .map((k) => ({ id: `${prefix}-${k}`, label: SECTION_TITLES[k], sclContext: prefix })),
  ]
}

export const sclTocItems = (input: SclDocument | SclBundle): SclTocItem[] => {
  if (!isSclBundle(input)) return singleSclTocItems(input)
  return [
    ...singleSclTocItems(input.root, 'overview'),
    ...input.contexts.flatMap((ctx) => contextTocItems(ctx)),
  ]
}
