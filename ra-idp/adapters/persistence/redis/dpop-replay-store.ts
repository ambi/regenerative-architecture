/**
 * Layer 4 — Adapter Layer (Redis DPoPReplayStore)
 *
 * jti のリプレイ検出。SET NX EX で「未観測なら記録、観測済みなら false」を
 * 1 ラウンドトリップで実行する (slo.yaml の introspect p99 50ms 要件に効く)。
 */

import type { DpopReplayStore } from '../../../src/oauth2/ports/dpop-replay-store'
import type { Redis } from './client'

const KEY_PREFIX = 'idp:dpop:jti:'

export class RedisDpopReplayStore implements DpopReplayStore {
  constructor(private readonly redis: Redis) {}

  async recordIfNew(jti: string, windowSeconds: number, _now: Date = new Date()): Promise<boolean> {
    // NX = only if not exists, EX = TTL in seconds
    // SET の戻りは新規セット時に 'OK'、既存時に null
    const result = await this.redis.set(KEY_PREFIX + jti, '1', 'EX', windowSeconds, 'NX')
    return result === 'OK'
  }
}
