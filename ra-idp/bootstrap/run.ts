/**
 * Layer 5 — Runtime: 起動シーケンスの統合。
 *
 * main.ts はこの run() を呼び出すだけ。新しい起動ステップを足すときは
 * 本ファイルに 1 行追加して順序を明示する。
 */

import type { Hono } from 'hono'

import { Argon2idPasswordHasher } from '../adapters/crypto/argon2id-password-hasher'

import { composeApp } from './app'
import { loadConfig } from './config'
import { assemble } from './dependencies'
import { createEmitter } from './emit'
import { assembleObserver } from './observer'
import { printStartupBanner } from './banner'
import { seedDemoData } from './seed'
import { registerShutdownHandlers } from './shutdown'

export interface RunResult {
  port: number
  fetch: Hono['fetch']
}

export async function run(): Promise<RunResult> {
  const config = loadConfig()
  const deps = await assemble(config)
  const observer = await assembleObserver(config, deps.eventSink)
  const emit = createEmitter(deps.eventSink)
  const passwordHasher = new Argon2idPasswordHasher()

  if (!process.env.SKIP_DEMO_SEED) {
    await seedDemoData(deps, passwordHasher)
  }

  const app = composeApp({ config, deps, observer, passwordHasher, emit })
  registerShutdownHandlers(config, observer)
  printStartupBanner(config)

  return { port: config.port, fetch: app.fetch }
}
