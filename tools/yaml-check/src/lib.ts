/**
 * yaml-check library — pure helpers used by the CLI (`main.ts`).
 *
 * Everything in this module is side-effect-free except for the Ajv compile
 * step (which runs once at import time). Tests target this file directly.
 */

import Ajv2020, { type ErrorObject, type ValidateFunction } from 'ajv/dist/2020.js'
import addFormats from 'ajv-formats'
import workItemSchema from '../schemas/work-item.schema.json' with { type: 'json' }
import sclSchema from '../schemas/scl.schema.json' with { type: 'json' }

export type Finding = { line: number; column: number; message: string }

const ajv = new Ajv2020({ allErrors: true, strict: false })
addFormats.default(ajv)

export const SCHEMAS: Record<string, ValidateFunction> = {
  'work-item': ajv.compile(workItemSchema),
  scl: ajv.compile(sclSchema),
}

export type CliOptions = {
  schema: string | null
  files: string[]
  listSchemas: boolean
  help: boolean
}

export type ArgsError = { kind: 'error'; code: number; message: string }
export type ArgsResult = { kind: 'ok'; opts: CliOptions } | ArgsError

export function parseArgs(argv: readonly string[]): ArgsResult {
  const files: string[] = []
  let schema: string | null = null
  let listSchemas = false
  let help = false
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i] ?? ''
    if (a === '--list-schemas') {
      listSchemas = true
    } else if (a === '--schema') {
      const next = argv[i + 1]
      if (next === undefined) {
        return { kind: 'error', code: 2, message: '--schema requires a value' }
      }
      schema = next
      i++
    } else if (a.startsWith('--schema=')) {
      schema = a.slice('--schema='.length)
    } else if (a === '--help' || a === '-h') {
      help = true
    } else if (a.startsWith('-')) {
      return { kind: 'error', code: 2, message: `unknown flag: ${a}` }
    } else {
      files.push(a)
    }
  }
  return { kind: 'ok', opts: { schema, files, listSchemas, help } }
}

export function lintRawText(text: string): Finding[] {
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
export function locatePointer(text: string, pointer: string): number {
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

export function decodeJsonPointerSegment(s: string): string {
  return s.replace(/~1/g, '/').replace(/~0/g, '~')
}

export function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function formatSchemaError(err: ErrorObject): string {
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

export function validateAgainstSchema(schemaName: string, data: unknown, text: string): Finding[] {
  const validate = SCHEMAS[schemaName]
  if (!validate) return []
  const ok = validate(data)
  if (ok || !validate.errors) return []
  return validate.errors.map((err) => ({
    line: locatePointer(text, err.instancePath),
    column: 1,
    message: formatSchemaError(err),
  }))
}
