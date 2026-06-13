/**
 * Layer 4 — Adapter Layer (in-memory LoginAttemptThrottle)
 *
 * fixed-window counter + lockout キーの 2 段。test / memory モード用。
 * 本番は Redis adapter (../redis/login-attempt-throttle.ts) を使う。詳細は ADR-029。
 */

import type {
  LoginAttemptThrottle,
  LoginThrottleAcquireResult,
  LoginThrottleKind,
} from '../../../src/authentication/ports/login-attempt-throttle'

export interface LoginThrottleConfig {
  maxFailures: number
  windowSeconds: number
  lockoutSeconds: number
}

export interface LoginThrottleConfigs {
  account: LoginThrottleConfig
  ip: LoginThrottleConfig
}

interface CounterEntry {
  failures: number
  windowExpiresAtMs: number
}

interface LockEntry {
  expiresAtMs: number
}

export class InMemoryLoginAttemptThrottle implements LoginAttemptThrottle {
  private readonly counters = new Map<string, CounterEntry>()
  private readonly locks = new Map<string, LockEntry>()

  constructor(private readonly configs: LoginThrottleConfigs) {}

  async tryAcquire(
    kind: LoginThrottleKind,
    key: string,
    now: Date,
  ): Promise<LoginThrottleAcquireResult> {
    const lock = this.locks.get(lockKey(kind, key))
    if (!lock) return { allowed: true }
    const remainingMs = lock.expiresAtMs - now.getTime()
    if (remainingMs <= 0) {
      this.locks.delete(lockKey(kind, key))
      return { allowed: true }
    }
    return { allowed: false, retryAfterSeconds: Math.ceil(remainingMs / 1000) }
  }

  async recordFailure(
    kind: LoginThrottleKind,
    key: string,
    now: Date,
  ): Promise<{ locked: boolean; retryAfterSeconds?: number }> {
    const config = this.configs[kind]
    const counterK = counterKey(kind, key)
    const lockK = lockKey(kind, key)
    const nowMs = now.getTime()

    const existing = this.counters.get(counterK)
    let entry: CounterEntry
    if (!existing || existing.windowExpiresAtMs <= nowMs) {
      entry = { failures: 1, windowExpiresAtMs: nowMs + config.windowSeconds * 1000 }
    } else {
      entry = { failures: existing.failures + 1, windowExpiresAtMs: existing.windowExpiresAtMs }
    }
    this.counters.set(counterK, entry)

    if (entry.failures >= config.maxFailures) {
      this.counters.delete(counterK)
      this.locks.set(lockK, { expiresAtMs: nowMs + config.lockoutSeconds * 1000 })
      return { locked: true, retryAfterSeconds: config.lockoutSeconds }
    }
    return { locked: false }
  }

  async recordSuccess(kind: LoginThrottleKind, key: string): Promise<void> {
    this.counters.delete(counterKey(kind, key))
    this.locks.delete(lockKey(kind, key))
  }
}

function counterKey(kind: LoginThrottleKind, key: string): string {
  return `c:${kind}:${key}`
}

function lockKey(kind: LoginThrottleKind, key: string): string {
  return `l:${kind}:${key}`
}
