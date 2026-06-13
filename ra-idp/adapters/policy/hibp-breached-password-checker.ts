/**
 * Layer 4 — Adapter Layer (HIBP Range API BreachedPasswordChecker)
 *
 * Have I Been Pwned の Pwned Passwords Range API を k-anonymity プロトコルで
 * 叩く実装。詳細は ADR-028。
 *
 * プロトコル:
 *   1. SHA-1(plain) → 40 文字 hex（大文字）
 *   2. prefix = 先頭 5 文字 / suffix = 残り 35 文字
 *   3. GET https://api.pwnedpasswords.com/range/{prefix} （Add-Padding: true）
 *   4. レスポンスは `SUFFIX:COUNT\r\n` の繰り返し。suffix 一致 & count > 0 で breached。
 *
 * fail-open: HTTP error / timeout / parse error はすべて false を返す。
 */

import { createHash } from 'crypto'
import type { BreachedPasswordChecker } from '../../src/authentication/ports/breached-password-checker'

const DEFAULT_ENDPOINT = 'https://api.pwnedpasswords.com/range'
const DEFAULT_TIMEOUT_MS = 2000

export interface HibpBreachedPasswordCheckerOptions {
  endpoint?: string
  timeoutMs?: number
  fetchImpl?: typeof fetch
}

export class HibpBreachedPasswordChecker implements BreachedPasswordChecker {
  private readonly endpoint: string
  private readonly timeoutMs: number
  private readonly fetchImpl: typeof fetch

  constructor(options: HibpBreachedPasswordCheckerOptions = {}) {
    this.endpoint = options.endpoint ?? DEFAULT_ENDPOINT
    this.timeoutMs = options.timeoutMs ?? DEFAULT_TIMEOUT_MS
    this.fetchImpl = options.fetchImpl ?? fetch
  }

  async isBreached(plain: string): Promise<boolean> {
    const sha1 = createHash('sha1').update(plain, 'utf8').digest('hex').toUpperCase()
    const prefix = sha1.slice(0, 5)
    const suffix = sha1.slice(5)

    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeoutMs)
    try {
      const res = await this.fetchImpl(`${this.endpoint}/${prefix}`, {
        method: 'GET',
        headers: { 'Add-Padding': 'true' },
        signal: controller.signal,
      })
      if (!res.ok) return false
      const text = await res.text()
      return containsSuffix(text, suffix)
    } catch {
      return false
    } finally {
      clearTimeout(timer)
    }
  }
}

function containsSuffix(body: string, suffix: string): boolean {
  for (const rawLine of body.split('\n')) {
    const line = rawLine.trim()
    if (line.length === 0) continue
    const sep = line.indexOf(':')
    if (sep <= 0) continue
    const lineSuffix = line.slice(0, sep).toUpperCase()
    if (lineSuffix !== suffix) continue
    const count = Number.parseInt(line.slice(sep + 1), 10)
    if (Number.isFinite(count) && count > 0) return true
  }
  return false
}
