/**
 * Layer 4 — Adapter Layer (Redis client wrapper)
 *
 * ioredis のラッパー。Redis を使わないテスト環境で `ioredis` の依存解決を
 * 発生させないために dynamic import を使う。
 */

type RedisCtor = any
type Redis = any

export interface RedisConfig {
  url: string // e.g. redis://localhost:6379
}

let cached: Redis | null = null

export async function getRedis(config: RedisConfig): Promise<Redis> {
  if (cached) return cached
  const mod = (await import('ioredis')) as any
  const RedisClass: RedisCtor = mod.default ?? mod.Redis
  cached = new RedisClass(config.url, {
    lazyConnect: false,
    maxRetriesPerRequest: 3,
    enableReadyCheck: true,
  })
  return cached
}

export async function closeRedis(): Promise<void> {
  if (cached) {
    await cached.quit()
    cached = null
  }
}

export type { Redis }
