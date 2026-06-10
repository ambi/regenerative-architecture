/**
 * Layer 4 — Adapter Layer (in-memory PasswordHistoryRepository)
 *
 * sub ごとに created_at DESC で履歴を保持。recent() は要求された depth 件まで返す。
 * テストでは depth が動的に変わるため、内部では剪定せず全件を保持する。
 */

import type {
  PasswordHistoryEntry,
  PasswordHistoryRepository,
} from '../../../src/authentication/ports/password-history-repository'

export class InMemoryPasswordHistoryRepository implements PasswordHistoryRepository {
  private readonly bySub = new Map<string, PasswordHistoryEntry[]>()

  async recent(sub: string, depth: number): Promise<PasswordHistoryEntry[]> {
    if (depth <= 0) return []
    const list = this.bySub.get(sub) ?? []
    return list.slice(0, depth).map((e) => ({ ...e }))
  }

  async add(sub: string, encoded: string, now: Date): Promise<void> {
    const list = this.bySub.get(sub) ?? []
    list.unshift({ encoded, created_at: now.toISOString() })
    this.bySub.set(sub, list)
  }
}
