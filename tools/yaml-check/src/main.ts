#!/usr/bin/env bun
/**
 * YAML check for the repository.
 *
 *   bun run yaml-check                       # default targets
 *   bun run yaml-check changes/foo/x.yaml    # explicit files / globs
 *
 * Two layers:
 *   1. Parse via Bun's built-in YAML loader (dynamic import) — same engine
 *      used by scl-to-html, so anything that parses here will parse there.
 *   2. Lint on the raw text: no tab indent, no trailing whitespace, must
 *      end with a single trailing newline.
 *
 * Exits non-zero if any target has a parse error or a lint violation.
 */

import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { isAbsolute, relative, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const REPO_ROOT = resolve(import.meta.dir, '../../..')

// Relative paths resolve against the shell cwd first, then fall back to the
// repo root. This way `bun --cwd tools yaml-check changes/foo.yaml` works
// whether invoked from the repo root or from tools/.
function resolvePath(p: string): string {
  if (isAbsolute(p)) return p
  const fromCwd = resolve(process.cwd(), p)
  if (existsSync(fromCwd)) return fromCwd
  return resolve(REPO_ROOT, p)
}

const DEFAULT_GLOBS = ['changes/**/*.yaml', 'ra-idp-go/spec/*.yaml', 'tools/**/spec/*.yaml']

type Finding = { line: number; column: number; message: string }

async function expandTargets(args: string[]): Promise<string[]> {
  const patterns = args.length > 0 ? args : DEFAULT_GLOBS
  const seen = new Set<string>()
  for (const pattern of patterns) {
    const isGlob = /[*?[]/.test(pattern)
    if (isGlob) {
      const glob = new Bun.Glob(pattern)
      for await (const match of glob.scan({ cwd: REPO_ROOT, absolute: true })) {
        if (!match.includes('/node_modules/')) seen.add(match)
      }
    } else {
      seen.add(resolvePath(pattern))
    }
  }
  return [...seen].sort()
}

async function parseYaml(path: string): Promise<Finding | null> {
  try {
    await import(pathToFileURL(path).href)
    return null
  } catch (e) {
    const err = e as { message?: string; line?: number; column?: number }
    return {
      line: err.line ?? 0,
      column: err.column ?? 0,
      message: err.message ?? String(e),
    }
  }
}

function lintRawText(text: string): Finding[] {
  const findings: Finding[] = []
  const lines = text.split('\n')
  const hasTrailingNewline = text.endsWith('\n')
  // text.split('\n') on "a\n" yields ["a", ""]; ignore the synthetic empty tail.
  const limit = hasTrailingNewline ? lines.length - 1 : lines.length
  for (let i = 0; i < limit; i++) {
    const raw = lines[i] ?? ''
    const indentMatch = raw.match(/^[\t ]*/)?.[0] ?? ''
    if (indentMatch.includes('\t')) {
      findings.push({
        line: i + 1,
        column: indentMatch.indexOf('\t') + 1,
        message: 'tab character in indentation',
      })
    }
    if (/[ \t]+$/.test(raw)) {
      findings.push({
        line: i + 1,
        column: raw.replace(/[ \t]+$/, '').length + 1,
        message: 'trailing whitespace',
      })
    }
  }
  if (!hasTrailingNewline && text.length > 0) {
    findings.push({ line: limit, column: 1, message: 'file does not end with a newline' })
  }
  if (text.endsWith('\n\n')) {
    findings.push({ line: limit, column: 1, message: 'file ends with multiple trailing newlines' })
  }
  return findings
}

function format(path: string, findings: Finding[]): string {
  const rel = relative(process.cwd(), path) || path
  return findings.map((f) => `${rel}:${f.line}:${f.column}: ${f.message}`).join('\n')
}

const args = process.argv.slice(2)
const targets = await expandTargets(args)

if (targets.length === 0) {
  console.error('yaml-check: no files matched')
  process.exit(args.length > 0 ? 1 : 0)
}

let failed = 0
for (const path of targets) {
  const text = await readFile(path, 'utf8')
  const parseError = await parseYaml(path)
  const lintFindings = lintRawText(text)
  const all: Finding[] = parseError ? [parseError, ...lintFindings] : lintFindings

  if (all.length === 0) {
    console.log(`ok  ${relative(process.cwd(), path) || path}`)
    continue
  }
  failed++
  console.log(`FAIL ${relative(process.cwd(), path) || path}`)
  process.stdout.write(`${format(path, all)}\n`)
}

if (failed > 0) {
  console.error(`\n${failed} file(s) failed (out of ${targets.length}).`)
  process.exit(1)
}
console.error(`\nAll ${targets.length} file(s) OK.`)
