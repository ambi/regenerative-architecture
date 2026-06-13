/**
 * Layer 4 — Adapter Layer (Redis LoginAttemptThrottle)
 *
 * fixed-window counter + lockout キーの 2 段。詳細は ADR-029。
 *
 * Redis key:
 *   idp:login:counter:{kind}:{key}  (INCR + EXPIRE window_seconds, 初回のみ EXPIRE)
 *   idp:login:lock:{kind}:{key}     (SET NX EX lockout_seconds)
 *
 * PTTL で残り秒数を返すことで Retry-After を即時に組み立てる。
 * counter は window 内 INCR のみ。lockout 設定後は同じ key を削除して再カウント開始。
 */

import type {
  LoginAttemptThrottle,
  LoginThrottleAcquireResult,
  LoginThrottleKind,
} from '../../../src/authentication/ports/login-attempt-throttle'
import type { LoginThrottleConfig, LoginThrottleConfigs } from '../memory/login-attempt-throttle'
import type { Redis } from './client'

const COUNTER_PREFIX = 'idp:login:counter:'
const LOCK_PREFIX = 'idp:login:lock:'

export class RedisLoginAttemptThrottle implements LoginAttemptThrottle {
  constructor(
    private readonly redis: Redis,
    private readonly configs: LoginThrottleConfigs,
  ) {}

  async tryAcquire(
    kind: LoginThrottleKind,
    key: string,
    _now: Date,
  ): Promise<LoginThrottleAcquireResult> {
    const lockK = LOCK_PREFIX + kind + ':' + key
    const pttl = await this.redis.pttl(lockK)
    if (pttl > 0) {
      return { allowed: false, retryAfterSeconds: Math.ceil(pttl / 1000) }
    }
    return { allowed: true }
  }

  async recordFailure(
    kind: LoginThrottleKind,
    key: string,
    _now: Date,
  ): Promise<{ locked: boolean; retryAfterSeconds?: number }> {
    const config = this.configs[kind]
    const counterK = COUNTER_PREFIX + kind + ':' + key
    const lockK = LOCK_PREFIX + kind + ':' + key

    const failures = await this.redis.incr(counterK)
    if (failures === 1) {
      await this.redis.expire(counterK, config.windowSeconds)
    }
    if (failures >= config.maxFailures) {
      await this.redis.set(lockK, '1', 'EX', config.lockoutSeconds)
      await this.redis.del(counterK)
      return { locked: true, retryAfterSeconds: config.lockoutSeconds }
    }
    return { locked: false }
  }

  async recordSuccess(kind: LoginThrottleKind, key: string): Promise<void> {
    const counterK = COUNTER_PREFIX + kind + ':' + key
    const lockK = LOCK_PREFIX + kind + ':' + key
    await this.redis.del(counterK, lockK)
  }
}

export type { LoginThrottleConfig, LoginThrottleConfigs }
