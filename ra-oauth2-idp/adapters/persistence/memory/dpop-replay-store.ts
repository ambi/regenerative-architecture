/**
 * Layer 4 — Adapter Layer (in-memory DPoPReplayStore)
 *
 * DPoP jti リプレイ防止用ストア。直近 10 分の jti を保持する。
 * 本番では Redis adapter (../redis/dpop-replay-store.ts) を使う。
 */

import type { DpopReplayStore } from '../../../src/ports/dpop-replay-store'

export class InMemoryDpopReplayStore implements DpopReplayStore {
  /** jti → 観測時刻(ms) */
  private readonly seen = new Map<string, number>()

  async recordIfNew(jti: string, windowSeconds: number, now: Date = new Date()): Promise<boolean> {
    this.gc(now, windowSeconds)
    if (this.seen.has(jti)) return false
    this.seen.set(jti, now.getTime())
    return true
  }

  private gc(now: Date, windowSeconds: number): void {
    const cutoff = now.getTime() - windowSeconds * 1000
    for (const [jti, t] of this.seen) {
      if (t < cutoff) this.seen.delete(jti)
    }
  }
}
