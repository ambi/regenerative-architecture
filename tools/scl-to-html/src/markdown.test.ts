import { describe, expect, it } from 'bun:test'
import { extractHeadings, firstParagraphText, renderMarkdown, splitTitle } from './markdown.ts'

describe('renderMarkdown', () => {
  it('escapes raw HTML in source', () => {
    const html = renderMarkdown('A <script>alert(1)</script> B')
    expect(html).not.toContain('<script>')
    expect(html).toContain('&lt;script&gt;')
  })

  it('renders headings, lists, and code blocks', () => {
    const html = renderMarkdown('# Title\n\n- item\n\n```ts\nconst x = 1\n```\n')
    expect(html).toContain('<h1>')
    expect(html).toContain('<ul>')
    expect(html).toContain('<pre>')
    expect(html).toContain('const x = 1')
  })

  it('wraps output in .md container so CSS can scope typography', () => {
    expect(renderMarkdown('hello')).toMatch(/^<div class="md">/)
  })
})

describe('splitTitle', () => {
  it('pulls the first level-1 heading and returns the rest as body', () => {
    const { title, body } = splitTitle('# Hello\n\nbody here', 'fb')
    expect(title).toBe('Hello')
    expect(body).toBe('body here')
  })
  it('falls back when no heading is present', () => {
    const { title, body } = splitTitle('no heading\n', 'fallback')
    expect(title).toBe('fallback')
    expect(body).toBe('no heading\n')
  })
})

describe('extractHeadings', () => {
  it('returns level / text / anchor for each # / ## / ###', () => {
    const headings = extractHeadings('# A\n## B B\n### C')
    expect(headings).toEqual([
      { level: 1, text: 'A', anchor: 'a' },
      { level: 2, text: 'B B', anchor: 'b-b' },
      { level: 3, text: 'C', anchor: 'c' },
    ])
  })
  it('ignores deeper headings and inline #', () => {
    const headings = extractHeadings('hash # in prose\n#### deep\n# top')
    expect(headings).toEqual([{ level: 1, text: 'top', anchor: 'top' }])
  })
})

describe('firstParagraphText', () => {
  it('returns the first non-empty paragraph as escaped plain text', () => {
    const text = firstParagraphText('# Title\n\nfirst para\n\nsecond para')
    expect(text).toBe('first para')
  })
  it('skips headings, lists, blockquotes and code fences', () => {
    const text = firstParagraphText('# Heading\n\n- item\n\n> quote\n\nreal para')
    expect(text).toBe('real para')
  })
  it('truncates to the given limit with an ellipsis', () => {
    const text = firstParagraphText('a'.repeat(500), 50)
    expect(text.endsWith('…')).toBe(true)
    expect(text.length).toBeLessThanOrEqual(51)
  })
  it('escapes HTML special chars', () => {
    expect(firstParagraphText('<x> & y')).toBe('&lt;x&gt; &amp; y')
  })
})
