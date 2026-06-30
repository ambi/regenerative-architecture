#!/usr/bin/env bun
/**
 * scl-to-jsonschema — generate JSON Schema 2020-12 from SCL `models`.
 *
 *   scl-to-jsonschema --scl <file> [--out <path>]
 *
 * The SCL document may be a context map (its contexts are loaded and merged)
 * or a single context file. Output is one schema document whose `$defs` holds
 * every model. Pure logic lives in `./generate.ts`; this file is the CLI shell.
 */

import { writeFile } from 'node:fs/promises'
import { loadSclBundle } from '../../scl-to-html/src/load.ts'
import { parseCliArgs } from './args.ts'
import { generateModelSchemas } from './generate.ts'

const result = parseCliArgs(process.argv.slice(2))
if (result.kind === 'help') {
  process.stdout.write(result.message)
  process.exit(0)
}
if (result.kind === 'error') {
  process.stderr.write(`scl-to-jsonschema: ${result.message}`)
  process.exit(result.code)
}

const bundle = await loadSclBundle(result.opts.scl)
const schema = generateModelSchemas(bundle)
const json = `${JSON.stringify(schema, null, 2)}\n`

if (result.opts.out) {
  await writeFile(result.opts.out, json, 'utf8')
  process.stderr.write(`wrote ${result.opts.out}\n`)
} else {
  process.stdout.write(json)
}
