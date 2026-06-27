import { describe, expect, it } from 'bun:test'
import {
  badge,
  chip,
  esc,
  humanConstraint,
  isObj,
  kvRow,
  link,
  renderAnnotations,
  renderConstraints,
  renderValue,
  slug,
  typeText,
} from './html.ts'

describe('esc', () => {
  it('escapes the five XML special chars', () => {
    expect(esc('<&>"\'')).toBe("&lt;&amp;&gt;&quot;'")
  })
  it('renders null / undefined as empty', () => {
    expect(esc(undefined)).toBe('')
    expect(esc(null)).toBe('')
  })
  it('coerces non-strings via String()', () => {
    expect(esc(42)).toBe('42')
    expect(esc(true)).toBe('true')
  })
})

describe('slug', () => {
  it('lowercases and replaces runs of non-alphanumerics with single dashes', () => {
    expect(slug('Hello World!!')).toBe('hello-world')
    expect(slug('  ABC ___ def  ')).toBe('abc-def')
  })
  it('strips leading and trailing dashes', () => {
    expect(slug('---foo---')).toBe('foo')
  })
  it('preserves digits', () => {
    expect(slug('ADR-024-key-rotation')).toBe('adr-024-key-rotation')
  })
  it('falls back to a stable non-empty slug for non-ASCII names', () => {
    expect(slug('管理者はアプリを作成できる')).toMatch(/^u-[a-z0-9]+$/)
    expect(slug('管理者はアプリを作成できる')).toBe(slug('管理者はアプリを作成できる'))
  })
})

describe('chip / link / badge', () => {
  it('chip wraps text with a span and optional kind class', () => {
    expect(chip('a')).toBe('<span class="chip">a</span>')
    expect(chip('a', 'kind')).toBe('<span class="chip chip-kind">a</span>')
  })
  it('link escapes the href and the body text', () => {
    expect(link('#a"b', '<x>', 'ref')).toBe(
      '<a class="chip chip-ref" href="#a&quot;b">&lt;x&gt;</a>',
    )
  })
  it('badge wraps text in a styled span', () => {
    expect(badge('on', 'k')).toBe('<span class="badge badge-k">on</span>')
  })
})

describe('typeText', () => {
  it('returns strings unchanged', () => {
    expect(typeText('UUID')).toBe('UUID')
  })
  it('returns "unknown" for null / undefined', () => {
    expect(typeText(null)).toBe('unknown')
    expect(typeText(undefined)).toBe('unknown')
  })
  it('JSON-stringifies object types', () => {
    expect(typeText({ map: 'String' })).toBe('{"map":"String"}')
  })
})

describe('kvRow', () => {
  it('renders a dt/dd pair', () => {
    expect(kvRow('k', 'v')).toBe('<dt>k</dt><dd>v</dd>')
  })
})

describe('isObj', () => {
  it('distinguishes plain objects from arrays and null', () => {
    expect(isObj({})).toBe(true)
    expect(isObj([])).toBe(false)
    expect(isObj(null)).toBe(false)
    expect(isObj('x')).toBe(false)
  })
})

describe('humanConstraint', () => {
  it('converts snake_case strings to spaces', () => {
    expect(humanConstraint('non_empty')).toBe('non empty')
  })
  it('formats object constraints as comma-separated pairs', () => {
    expect(humanConstraint({ max_length: 200 })).toBe('max length: 200')
  })
})

describe('renderConstraints / renderAnnotations', () => {
  it('emits one chip per constraint', () => {
    const html = renderConstraints(['non_empty', { max_length: 100 }])
    expect(html).toContain('non empty')
    expect(html).toContain('max length: 100')
  })
  it('skips annotations whose value is false', () => {
    const html = renderAnnotations({ sensitive: true, pii: false })
    expect(html).toContain('sensitive')
    expect(html).not.toContain('pii')
  })
  it('returns empty for non-objects', () => {
    expect(renderAnnotations(undefined)).toBe('')
    expect(renderAnnotations('x')).toBe('')
  })
})

describe('renderValue', () => {
  it('renders null / undefined as a muted dash', () => {
    expect(renderValue(undefined)).toContain('—')
  })
  it('renders primitive scalars wrapped in code', () => {
    expect(renderValue(42)).toBe('<code>42</code>')
    expect(renderValue(true)).toBe('<code>true</code>')
  })
  it('renders flat string arrays as chip rows', () => {
    expect(renderValue(['a', 'b'])).toContain('chip')
  })
  it('renders object arrays as nested lists', () => {
    const html = renderValue([{ x: 1 }, { y: 2 }])
    expect(html).toContain('<ul class="vlist">')
  })
  it('renders objects as kv lists', () => {
    expect(renderValue({ a: 1 })).toContain('<dl class="kv">')
  })
  it('escapes strings', () => {
    expect(renderValue('<x>')).toContain('&lt;x&gt;')
  })
})
