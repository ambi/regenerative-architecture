/**
 * CLI argument parsing for scl-to-openapi. Pure — no IO, no exits.
 */

export type CliOptions = {
  scl: string
  out: string | null
}

export type ParseResult =
  | { kind: 'ok'; opts: CliOptions }
  | { kind: 'help'; message: string }
  | { kind: 'error'; code: number; message: string }

const USAGE = [
  'Usage: scl-to-openapi --scl <file> [--out <path>]',
  '       scl-to-openapi --help',
  '',
  '  --scl   SCL YAML document (a context map or a single context) (required).',
  '  --out   Output OpenAPI 3.1 JSON path. Without --out it is written to stdout.',
  '',
].join('\n')

const FLAGS_WITH_VALUE = new Set(['--scl', '--out'])

export function parseCliArgs(argv: readonly string[]): ParseResult {
  const opts: CliOptions = { scl: '', out: null }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i] ?? ''
    if (a === '--help' || a === '-h') return { kind: 'help', message: USAGE }
    const eq = a.indexOf('=')
    let name: string
    let value: string | undefined
    if (a.startsWith('--') && eq !== -1) {
      name = a.slice(0, eq)
      value = a.slice(eq + 1)
    } else if (FLAGS_WITH_VALUE.has(a)) {
      name = a
      value = argv[i + 1]
      if (value === undefined) {
        return { kind: 'error', code: 2, message: `${a} requires a value\n${USAGE}` }
      }
      i++
    } else {
      return { kind: 'error', code: 2, message: `unexpected argument: ${a}\n${USAGE}` }
    }
    switch (name) {
      case '--scl':
        opts.scl = value ?? ''
        break
      case '--out':
        opts.out = value ?? null
        break
      default:
        return { kind: 'error', code: 2, message: `unknown flag: ${name}\n${USAGE}` }
    }
  }
  if (!opts.scl) return { kind: 'error', code: 2, message: `--scl is required\n${USAGE}` }
  return { kind: 'ok', opts }
}
