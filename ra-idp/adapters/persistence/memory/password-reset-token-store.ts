/**
 * Layer 4 — Adapter Layer (in-memory PasswordResetTokenStore)
 *
 * ADR-030: 単発消費 + 同 sub の旧 token 失効。test / memory モード用。
 */

import type {
  PasswordResetTokenRecord,
  PasswordResetTokenStore,
} from '../../../src/authentication/ports/password-reset-token-store'

export class InMemoryPasswordResetTokenStore implements PasswordResetTokenStore {
  private readonly byTokenHash = new Map<string, PasswordResetTokenRecord>()

  async save(record: PasswordResetTokenRecord): Promise<void> {
    // 同 sub の未消費を失効: 該当行を全削除する。
    for (const [hash, existing] of this.byTokenHash) {
      if (existing.sub === record.sub) this.byTokenHash.delete(hash)
    }
    this.byTokenHash.set(record.token_hash, { ...record })
  }

  async consume(tokenHash: string, now: Date): Promise<PasswordResetTokenRecord | null> {
    const record = this.byTokenHash.get(tokenHash)
    if (!record) return null
    // 期限切れも消費済み相当として扱い、行は削除する。
    this.byTokenHash.delete(tokenHash)
    if (Date.parse(record.expires_at) <= now.getTime()) return null
    return record
  }
}
