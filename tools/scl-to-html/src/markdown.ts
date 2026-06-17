/**
 * Markdown → HTML adapter.
 *
 * Wraps markdown-it with a conservative configuration:
 *   - `html: false`  — raw HTML in markdown is escaped, so an ADR or
 *     CONCEPTION file cannot inject scripts or break the page shell.
 *   - `linkify: false` — only explicit Markdown links become anchors;
 *     bare URLs in prose stay literal (less surprise in technical text).
 *   - GFM-style tables and fenced code blocks.
 */

import MarkdownIt from 'markdown-it'
import { esc, slug } from './html.ts'

const md = new MarkdownIt({
  html: false,
  linkify: false,
  typographer: false,
  breaks: false,
})

/**
 * Render markdown to HTML. The wrapper div lets the page CSS scope
 * typography rules so they don't leak into the SCL-card layout.
 */
export const renderMarkdown = (source: string): string =>
  `<div class="md">${md.render(source)}</div>`

/**
 * Pull the first level-1 heading out of a markdown document and return
 * the title plus the remaining body. Falls back to `fallback` when no
 * heading is present (e.g. files that start straight with prose).
 */
export const splitTitle = (source: string, fallback: string): { title: string; body: string } => {
  const match = source.match(/^[\s\n]*#\s+(.+?)\s*\n/)
  if (!match) return { title: fallback, body: source }
  const title = match[1]?.trim() ?? fallback
  const body = source.slice(match[0].length)
  return { title, body }
}

/**
 * Heading -> { level, text, anchor } records, used to build per-document
 * TOCs in the Decisions tab. Picks up `#`, `##`, and `###` headings.
 */
export const extractHeadings = (
  source: string,
): Array<{ level: number; text: string; anchor: string }> => {
  const out: Array<{ level: number; text: string; anchor: string }> = []
  for (const line of source.split('\n')) {
    const m = line.match(/^(#{1,3})\s+(.+?)\s*$/)
    if (!m) continue
    const text = m[2]?.trim() ?? ''
    out.push({ level: (m[1] ?? '').length, text, anchor: slug(text) })
  }
  return out
}

/** Render the first paragraph of `source` as plain text, capped at `limit`. */
export const firstParagraphText = (source: string, limit = 280): string => {
  const lines = source.split('\n')
  const collected: string[] = []
  for (const line of lines) {
    if (!line.trim()) {
      if (collected.length > 0) break
      continue
    }
    if (/^#{1,6}\s/.test(line)) continue
    if (/^(```|~~~|\||>|[-*+]\s|\d+\.\s)/.test(line)) continue
    collected.push(line.trim())
    if (collected.join(' ').length >= limit) break
  }
  const joined = collected.join(' ')
  if (joined.length <= limit) return esc(joined)
  return `${esc(joined.slice(0, limit).trimEnd())}…`
}
