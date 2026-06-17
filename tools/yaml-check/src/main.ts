#!/usr/bin/env bun
/**
 * YAML check for the repository.
 *
 *   yaml-check <file>...                          # parse + lint only
 *   yaml-check --schema=<name> <file>...          # parse + lint + schema
 *   yaml-check --list-schemas                     # list available schema names
 *
 * Three layers:
 *   1. Parse via Bun's built-in YAML loader (dynamic import) — same engine
 *      used by scl-to-html, so anything that parses here will parse there.
 *   2. Lint on the raw text: no tab indent, no trailing whitespace, must
 *      end with a single trailing newline.
 *   3. (opt-in) JSON Schema 2020-12 validation via Ajv. Schemas are
 *      explicit, never inferred from filename — a chance basename collision
 *      should not silently activate a schema unrelated to the file.
 *
 * Exits non-zero if any target has a parse error, a lint violation, or a
 * schema violation.
 */

import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { isAbsolute, relative, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import Ajv2020, { type ErrorObject, type ValidateFunction } from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import workItemSchema from '../schemas/work-item.schema.json' with { type: 'json' }
import completionReportSchema from '../schemas/completion-report.schema.json' with { type: 'json' }
import sclSchema from '../schemas/scl.schema.json' with { type: 'json' }

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

type Finding = { line: number; column: number; message: string }

const ajv = new Ajv2020({ allErrors: true, strict: false })
addFormats.default(ajv)
const SCHEMAS: Record<string, ValidateFunction> = {
  'work-item': ajv.compile(workItemSchema),
  'completion-report': ajv.compile(completionReportSchema),
  scl: ajv.compile(sclSchema),
}

type CliOptions = { schema: string | null; files: string[]; listSchemas: boolean }

function parseArgs(argv: string[]): CliOptions {
  const files: string[] = []
  let schema: string | null = null
  let listSchemas = false
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i] ?? ''
    if (a === '--list-schemas') {
      listSchemas = true
    } else if (a === '--schema') {
      const next = argv[i + 1]
      if (next === undefined) {
        console.error('yaml-check: --schema requires a value')
        process.exit(2)
      }
      schema = next
      i++
    } else if (a.startsWith('--schema=')) {
      schema = a.slice('--schema='.length)
    } else if (a === '--help' || a === '-h') {
      printUsage()
      process.exit(0)
    } else if (a.startsWith('-')) {
      console.error(`yaml-check: unknown flag: ${a}`)
      process.exit(2)
    } else {
      files.push(a)
    }
  }
  return { schema, files, listSchemas }
}

function printUsage(): void {
  process.stdout.write(
    [
      'Usage: yaml-check [--schema=<name>] <file-or-glob>...',
      '       yaml-check --list-schemas',
      '',
      'Without --schema, only YAML parse + raw-text lint runs.',
      'With --schema, the named JSON Schema is applied to every input file.',
      `Available schemas: ${Object.keys(SCHEMAS).join(', ')}`,
      '',
    ].join('\n'),
  )
}

async function expandTargets(patterns: string[]): Promise<string[]> {
  const seen = new Set<string>()
  for (const pattern of patterns) {
    const isGlob = /[*?[]/.test(pattern)
    if (isGlob) {
      // Resolve the glob against the shell cwd first (matches what the user
      // typed), then fall back to the repo root if nothing matched. Glob
      // patterns can contain `..` so we cannot pass them to Bun.Glob with a
      // mismatched cwd.
      let matched = 0
      const tryScan = async (cwd: string): Promise<void> => {
        const glob = new Bun.Glob(pattern)
        for await (const match of glob.scan({ cwd, absolute: true })) {
          if (!match.includes('/node_modules/')) {
            seen.add(match)
            matched++
          }
        }
      }
      await tryScan(process.cwd())
      if (matched === 0 && process.cwd() !== REPO_ROOT) await tryScan(REPO_ROOT)
    } else {
      seen.add(resolvePath(pattern))
    }
  }
  return [...seen].sort()
}

type ParseResult = { ok: true; data: unknown } | { ok: false; finding: Finding }

async function parseYaml(path: string): Promise<ParseResult> {
  try {
    const mod = await import(pathToFileURL(path).href)
    return { ok: true, data: mod.default }
  } catch (e) {
    const err = e as { message?: string; line?: number; column?: number }
    return {
      ok: false,
      finding: {
        line: err.line ?? 0,
        column: err.column ?? 0,
        message: err.message ?? String(e),
      },
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

// Map a JSON pointer like "/scope/ui/pages/0" to the (1-based) line in the
// source text. Pure heuristic: walk keys top-down, find the first indented
// occurrence of each key after the previous match. Returns 1 when nothing
// matches.
function locatePointer(text: string, pointer: string): number {
  if (!pointer) return 1
  const segments = pointer
    .split('/')
    .filter((s) => s.length > 0)
    .map(decodeJsonPointerSegment)
  const lines = text.split('\n')
  let cursor = 0
  for (const seg of segments) {
    if (/^\d+$/.test(seg)) {
      // Array index: count list items at the current indent below `cursor`.
      const idx = Number.parseInt(seg, 10)
      const indent = (lines[cursor] ?? '').match(/^[ ]*/)?.[0].length ?? 0
      let seen = -1
      for (let i = cursor + 1; i < lines.length; i++) {
        const m = (lines[i] ?? '').match(/^([ ]*)-(\s|$)/)
        if (m && m[1] !== undefined && m[1].length >= indent) {
          seen += 1
          if (seen === idx) return i + 1
        }
      }
      return cursor + 1
    }
    const re = new RegExp(`^\\s*${escapeRegExp(seg)}\\s*:`)
    let found = -1
    for (let i = cursor; i < lines.length; i++) {
      if (re.test(lines[i] ?? '')) {
        found = i
        break
      }
    }
    if (found < 0) return cursor + 1
    cursor = found
  }
  return cursor + 1
}

function decodeJsonPointerSegment(s: string): string {
  return s.replace(/~1/g, '/').replace(/~0/g, '~')
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function formatSchemaError(err: ErrorObject): string {
  const path = err.instancePath || '/'
  const detail =
    err.keyword === 'additionalProperties' && typeof err.params.additionalProperty === 'string'
      ? ` (${err.params.additionalProperty})`
      : err.keyword === 'enum' && Array.isArray(err.params.allowedValues)
        ? ` (allowed: ${err.params.allowedValues.join(', ')})`
        : err.keyword === 'required' && typeof err.params.missingProperty === 'string'
          ? ` (missing: ${err.params.missingProperty})`
          : ''
  return `schema: ${path} ${err.message ?? ''}${detail}`.trimEnd()
}

function formatFindings(path: string, findings: Finding[]): string {
  const rel = relative(process.cwd(), path) || path
  return findings.map((f) => `${rel}:${f.line}:${f.column}: ${f.message}`).join('\n')
}

const opts = parseArgs(process.argv.slice(2))

if (opts.listSchemas) {
  for (const name of Object.keys(SCHEMAS)) console.log(name)
  process.exit(0)
}

let validate: ValidateFunction | null = null
if (opts.schema !== null) {
  validate = SCHEMAS[opts.schema] ?? null
  if (validate === null) {
    console.error(
      `yaml-check: unknown schema '${opts.schema}'. Available: ${Object.keys(SCHEMAS).join(', ')}`,
    )
    process.exit(2)
  }
}

if (opts.files.length === 0) {
  console.error('yaml-check: no input files given')
  printUsage()
  process.exit(2)
}

const targets = await expandTargets(opts.files)

if (targets.length === 0) {
  console.error('yaml-check: no files matched')
  process.exit(1)
}

let failed = 0
for (const path of targets) {
  const text = await readFile(path, 'utf8')
  const parseResult = await parseYaml(path)
  const lintFindings = lintRawText(text)
  const findings: Finding[] = []
  if (!parseResult.ok) findings.push(parseResult.finding)
  findings.push(...lintFindings)

  if (parseResult.ok && validate !== null) {
    const valid = validate(parseResult.data)
    if (!valid && validate.errors) {
      for (const err of validate.errors) {
        findings.push({
          line: locatePointer(text, err.instancePath),
          column: 1,
          message: formatSchemaError(err),
        })
      }
    }
  }

  if (findings.length === 0) {
    console.log(`ok  ${relative(process.cwd(), path) || path}`)
    continue
  }
  failed++
  console.log(`FAIL ${relative(process.cwd(), path) || path}`)
  process.stdout.write(`${formatFindings(path, findings)}\n`)
}

if (failed > 0) {
  console.error(`\n${failed} file(s) failed (out of ${targets.length}).`)
  process.exit(1)
}
console.error(`\nAll ${targets.length} file(s) OK.`)
