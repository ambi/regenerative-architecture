/**
 * Layer 4 — Adapter: Redis AccessTokenDenylist。
 *
 * SET <key> 1 EX <ttl> で jti を記録し、EX で自動消去させる。
 * 検索は EXISTS 1 ラウンドトリップ (slo.yaml introspect p99 50ms 要件に乗る)。
 */

import type { AccessTokenDenylist } from '../../../src/oauth2/ports/access-token-denylist'
import type { Redis } from './client'

const KEY_PREFIX = 'idp:at:denylist:'

export class RedisAccessTokenDenylist implements AccessTokenDenylist {
  constructor(private readonly redis: Redis) {}

  async add(jti: string, expiresAt: Date): Promise<void> {
    const ttlSeconds = Math.max(1, Math.ceil((expiresAt.getTime() - Date.now()) / 1000))
    await this.redis.set(KEY_PREFIX + jti, '1', 'EX', ttlSeconds)
  }

  async isRevoked(jti: string): Promise<boolean> {
    const v = await this.redis.exists(KEY_PREFIX + jti)
    return v === 1
  }
}
