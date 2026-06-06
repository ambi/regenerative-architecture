/**
 * Layer 4 — Adapter Layer (Redis SessionStore)
 */

import type Redis from 'ioredis'
import type { LoginSession, SessionStore } from '../../../src/authentication/ports/session-store'

export class RedisSessionStore implements SessionStore {
  constructor(
    private readonly redis: Redis,
    private readonly ttlSeconds = 3600,
  ) {}

  private key(id: string): string {
    return `login_session:${id}`
  }

  async find(id: string): Promise<LoginSession | null> {
    const raw = await this.redis.get(this.key(id))
    if (!raw) return null
    const session = JSON.parse(raw) as LoginSession
    if (Date.parse(session.expires_at) <= Date.now()) {
      await this.delete(id)
      return null
    }
    return session
  }

  async save(session: LoginSession): Promise<void> {
    await this.redis.set(this.key(session.id), JSON.stringify(session), 'EX', this.ttlSeconds)
  }

  async delete(id: string): Promise<void> {
    await this.redis.del(this.key(id))
  }
}
