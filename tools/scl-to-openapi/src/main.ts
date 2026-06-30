#!/usr/bin/env bun
/**
 * scl-to-openapi — generate an OpenAPI 3.1 document from SCL.
 *
 *   scl-to-openapi --scl <file> [--out <path>]
 *
 * Interfaces with http bindings become operations; models become
 * components/schemas. Pure logic lives in `./openapi.ts`; this is the shell.
 */

import { writeFile } from 'node:fs/promises'
import { loadSclBundle } from '../../scl-to-html/src/load.ts'
import { parseCliArgs } from './args.ts'
import { generateOpenApi } from './openapi.ts'

const result = parseCliArgs(process.argv.slice(2))
if (result.kind === 'help') {
  process.stdout.write(result.message)
  process.exit(0)
}
if (result.kind === 'error') {
  process.stderr.write(`scl-to-openapi: ${result.message}`)
  process.exit(result.code)
}

const bundle = await loadSclBundle(result.opts.scl)
const doc = generateOpenApi(bundle)
const json = `${JSON.stringify(doc, null, 2)}\n`

if (result.opts.out) {
  await writeFile(result.opts.out, json, 'utf8')
  process.stderr.write(`wrote ${result.opts.out}\n`)
} else {
  process.stdout.write(json)
}
