/**
 * Layer 4 — Adapter: InMemory AccessTokenDenylist。
 *
 * 検査時に期限切れエントリを掃除する lazy GC。
 */

import type { AccessTokenDenylist } from '../../../src/oauth2/ports/access-token-denylist'

export class InMemoryAccessTokenDenylist implements AccessTokenDenylist {
  private readonly entries = new Map<string, number>()

  async add(jti: string, expiresAt: Date): Promise<void> {
    this.entries.set(jti, expiresAt.getTime())
  }

  async isRevoked(jti: string): Promise<boolean> {
    this.sweep()
    return this.entries.has(jti)
  }

  private sweep(now: number = Date.now()): void {
    for (const [jti, exp] of this.entries) {
      if (exp <= now) this.entries.delete(jti)
    }
  }
}
