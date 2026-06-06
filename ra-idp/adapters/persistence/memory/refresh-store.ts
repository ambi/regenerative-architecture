/**
 * Layer 4 — Adapter Layer (in-memory RefreshTokenStore)
 *
 * family_id ベースの効率的失効をサポートする。
 * 本番では Postgres adapter (../postgres/refresh-token-store.ts) を使う。
 */

import type { RefreshTokenRecord } from '../../../src/spec-bindings/schemas'
import type { RefreshTokenStore } from '../../../src/oauth2/ports/refresh-token-store'

export class InMemoryRefreshTokenStore implements RefreshTokenStore {
  private readonly byHash = new Map<string, RefreshTokenRecord>()
  private readonly byId = new Map<string, RefreshTokenRecord>()
  /** family_id → set of token ids */
  private readonly byFamily = new Map<string, Set<string>>()

  async findByHash(hash: string): Promise<RefreshTokenRecord | null> {
    const r = this.byHash.get(hash)
    return r ? { ...r } : null
  }

  async save(record: RefreshTokenRecord): Promise<void> {
    this.byHash.set(record.hash, { ...record })
    this.byId.set(record.id, { ...record })
    let family = this.byFamily.get(record.family_id)
    if (!family) {
      family = new Set()
      this.byFamily.set(record.family_id, family)
    }
    family.add(record.id)
  }

  async rotate(
    parentId: string,
    newRecord: RefreshTokenRecord,
  ): Promise<RefreshTokenRecord | null> {
    const parent = this.byId.get(parentId)
    if (!parent) return null
    if (parent.rotated || parent.revoked) return null
    const rotatedParent = { ...parent, rotated: true }
    this.byId.set(parent.id, rotatedParent)
    this.byHash.set(parent.hash, rotatedParent)
    await this.save(newRecord)
    return newRecord
  }

  async revokeFamily(family_id: string): Promise<void> {
    const ids = this.byFamily.get(family_id)
    if (!ids) return
    for (const id of ids) {
      const r = this.byId.get(id)
      if (r) {
        const revoked = { ...r, revoked: true }
        this.byId.set(id, revoked)
        this.byHash.set(r.hash, revoked)
      }
    }
  }
}
