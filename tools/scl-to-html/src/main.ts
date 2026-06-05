#!/usr/bin/env bun
/**
 * CLI driver for Render. Reads YAML via Bun's built-in YAML import.
 *
 * Usage: bun run src/main.ts <input.yaml> [output.html]
 */

import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { render, type SclDocument } from './render.ts'

const [, , inputArg, outputArg] = process.argv
if (!inputArg) {
  console.error('Usage: scl-to-html <input.yaml> [output.html]')
  process.exit(1)
}

const inputPath = resolve(process.cwd(), inputArg)
const mod = await import(pathToFileURL(inputPath).href)
const scl = ((mod as { default?: unknown }).default ?? mod) as SclDocument

const html = render(scl)

if (outputArg) {
  const outputPath = resolve(process.cwd(), outputArg)
  await mkdir(dirname(outputPath), { recursive: true })
  await writeFile(outputPath, html)
  console.error(`Wrote ${outputArg}`)
} else {
  process.stdout.write(html)
}
