/**
 * CLI argument parsing for scl-to-html. Pure function — no IO, no exits.
 *
 * Returns one of three results: `help` (print and exit 0), `error` (print
 * to stderr and exit non-zero), or `ok` with normalized options.
 */

export type CliOptions = {
  scl: string
  decisions: string | null
  changes: string | null
  out: string | null
  title: string | null
}

export type ParseResult =
  | { kind: 'ok'; opts: CliOptions }
  | { kind: 'help'; message: string }
  | { kind: 'error'; code: number; message: string }

const USAGE = [
  'Usage: scl-to-html --scl <file> [--decisions <dir>] [--changes <dir>]',
  '                   [--title <string>] [--out <path>]',
  '       scl-to-html --help',
  '',
  '  --scl        SCL YAML document (required).',
  '  --decisions  Directory with CONCEPTION*.md + ADR-*.md (optional).',
  '  --changes    Directory with <id>/{work-item,completion-report}.yaml (optional).',
  '  --title      Override the page <title> and header (defaults to the SCL system name).',
  '  --out        Output HTML path. Without --out the HTML is written to stdout.',
  '',
].join('\n')

const FLAGS_WITH_VALUE = new Set(['--scl', '--decisions', '--changes', '--title', '--out'])

export function parseCliArgs(argv: readonly string[]): ParseResult {
  const opts: CliOptions = {
    scl: '',
    decisions: null,
    changes: null,
    out: null,
    title: null,
  }
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i] ?? ''
    if (a === '--help' || a === '-h') {
      return { kind: 'help', message: USAGE }
    }
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
    } else if (a.startsWith('-')) {
      return { kind: 'error', code: 2, message: `unknown flag: ${a}\n${USAGE}` }
    } else {
      return {
        kind: 'error',
        code: 2,
        message: `unexpected positional argument: ${a}\n${USAGE}`,
      }
    }

    switch (name) {
      case '--scl':
        opts.scl = value ?? ''
        break
      case '--decisions':
        opts.decisions = value ?? null
        break
      case '--changes':
        opts.changes = value ?? null
        break
      case '--title':
        opts.title = value ?? null
        break
      case '--out':
        opts.out = value ?? null
        break
      default:
        return { kind: 'error', code: 2, message: `unknown flag: ${name}\n${USAGE}` }
    }
  }
  if (!opts.scl) {
    return { kind: 'error', code: 2, message: `--scl is required\n${USAGE}` }
  }
  return { kind: 'ok', opts }
}
