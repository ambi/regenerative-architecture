/**
 * Layer 4 — Adapter Layer (Redis ClientAssertionReplayStore)
 *
 * private_key_jwt の client_assertion jti リプレイ検出。
 * SET NX EX で「未観測なら記録、観測済みなら false」を 1 ラウンドトリップで実行する。
 * DPoP jti とは別名前空間 (idp:cassert:jti:) を使う。
 */

import type { ClientAssertionReplayStore } from '../../../src/ports/client-assertion-replay-store'
import type { Redis } from './client'

const KEY_PREFIX = 'idp:cassert:jti:'

export class RedisClientAssertionReplayStore implements ClientAssertionReplayStore {
  constructor(private readonly redis: Redis) {}

  async recordIfNew(jti: string, windowSeconds: number, _now: Date = new Date()): Promise<boolean> {
    const result = await this.redis.set(KEY_PREFIX + jti, '1', 'EX', windowSeconds, 'NX')
    return result === 'OK'
  }
}
