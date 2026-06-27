/**
 * HTML primitives shared across renderers.
 *
 * Everything is pure: no DOM, no IO, no clock. Strings only.
 */

/** HTML-escape any value for safe interpolation into element bodies / attribute strings. */
export const esc = (s: unknown): string =>
  String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')

/** Normalize an arbitrary string into a stable kebab-case anchor fragment. */
export const slug = (s: string): string => {
  const normalized = s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
  if (normalized) return normalized

  let hash = 2166136261
  for (const ch of s) {
    hash ^= ch.codePointAt(0) ?? 0
    hash = Math.imul(hash, 16777619)
  }
  return `u-${(hash >>> 0).toString(36)}`
}

export const isObj = (v: unknown): v is Record<string, unknown> =>
  v !== null && typeof v === 'object' && !Array.isArray(v)

export const chip = (text: unknown, kind = ''): string =>
  `<span class="chip${kind ? ` chip-${kind}` : ''}">${esc(text)}</span>`

export const link = (href: string, text: unknown, kind = ''): string =>
  `<a class="chip${kind ? ` chip-${kind}` : ''}" href="${esc(href)}">${esc(text)}</a>`

export const badge = (text: unknown, kind = ''): string =>
  `<span class="badge${kind ? ` badge-${kind}` : ''}">${esc(text)}</span>`

export const typeText = (t: unknown): string =>
  typeof t === 'string' ? t : t === undefined || t === null ? 'unknown' : JSON.stringify(t)

export const kvRow = (k: string, v: string): string => `<dt>${esc(k)}</dt><dd>${v}</dd>`

/** Render an arbitrary JSON-ish value as nested kv lists, chip rows, or text. */
export const renderValue = (v: unknown): string => {
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

export const humanConstraint = (c: unknown): string => {
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

export const renderConstraints = (cs: unknown): string =>
  Array.isArray(cs) ? cs.map((c) => chip(humanConstraint(c), 'constraint')).join(' ') : ''

export const renderAnnotations = (ann: unknown): string => {
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
