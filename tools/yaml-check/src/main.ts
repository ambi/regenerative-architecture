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
 * Pure logic lives in `./lib.ts`; this file is the CLI shell only.
 *
 * Exits non-zero if any target has a parse error, a lint violation, or a
 * schema violation.
 */

import { existsSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { isAbsolute, relative, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { type Finding, SCHEMAS, lintRawText, parseArgs, validateAgainstSchema } from './lib.ts'

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

function formatFindings(path: string, findings: Finding[]): string {
  const rel = relative(process.cwd(), path) || path
  return findings.map((f) => `${rel}:${f.line}:${f.column}: ${f.message}`).join('\n')
}

const argsResult = parseArgs(process.argv.slice(2))
if (argsResult.kind === 'error') {
  console.error(`yaml-check: ${argsResult.message}`)
  process.exit(argsResult.code)
}
const opts = argsResult.opts

if (opts.help) {
  printUsage()
  process.exit(0)
}

if (opts.listSchemas) {
  for (const name of Object.keys(SCHEMAS)) console.log(name)
  process.exit(0)
}

if (opts.schema !== null && !(opts.schema in SCHEMAS)) {
  console.error(
    `yaml-check: unknown schema '${opts.schema}'. Available: ${Object.keys(SCHEMAS).join(', ')}`,
  )
  process.exit(2)
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

  if (parseResult.ok && opts.schema !== null) {
    findings.push(...validateAgainstSchema(opts.schema, parseResult.data, text))
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
