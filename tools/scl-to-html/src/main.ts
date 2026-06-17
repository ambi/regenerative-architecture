#!/usr/bin/env bun
/**
 * scl-to-html CLI.
 *
 *   scl-to-html --scl <path/to/scl.yaml>
 *               [--decisions <dir>]
 *               [--changes <dir>]
 *               [--title <string>]
 *               [--out <path>]
 *
 * --decisions and --changes are optional. Without them the produced HTML
 * still has the Overview and SCL tabs; the Decisions / Changes tabs render
 * an "empty" placeholder.
 *
 * Without --out the HTML is written to stdout.
 *
 * Pure logic lives in src/render-*.ts and src/page.ts; this file is the
 * CLI shell (argument parsing, IO).
 */

import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { parseCliArgs } from './args.ts'
import { loadChanges, loadDecisions, loadScl } from './load.ts'
import { renderPage } from './page.ts'

const argv = process.argv.slice(2)
const parsed = parseCliArgs(argv)
if (parsed.kind === 'help') {
  process.stdout.write(parsed.message)
  process.exit(0)
}
if (parsed.kind === 'error') {
  process.stderr.write(`scl-to-html: ${parsed.message}\n`)
  process.exit(parsed.code)
}

const { scl: sclArg, decisions: decisionsArg, changes: changesArg, out, title } = parsed.opts

const sclPath = resolve(process.cwd(), sclArg)
const decisionsPath = decisionsArg ? resolve(process.cwd(), decisionsArg) : null
const changesPath = changesArg ? resolve(process.cwd(), changesArg) : null

const scl = await loadScl(sclPath)
const decisions = decisionsPath ? await loadDecisions(decisionsPath) : []
const changes = changesPath ? await loadChanges(changesPath) : []

const html = renderPage({ scl, decisions, changes, title: title ?? undefined })

if (out) {
  const outPath = resolve(process.cwd(), out)
  await mkdir(dirname(outPath), { recursive: true })
  await writeFile(outPath, html)
  process.stderr.write(`Wrote ${out}\n`)
} else {
  process.stdout.write(html)
}
