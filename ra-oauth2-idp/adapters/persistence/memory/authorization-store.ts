/**
 * Layer 4 — Adapter Layer (in-memory volatile stores)
 *
 * 認可リクエスト・認可コード・PAR の一時保存 (memory implementation)。
 * 本番では Redis adapter (../redis/) を使う。
 */

import type {
  AuthorizationRequest,
  AuthorizationCode,
  PARRecord,
} from '../../../src/spec-bindings/schemas'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from '../../../src/ports/authorization-store'

export class InMemoryAuthorizationRequestStore implements AuthorizationRequestStore {
  private readonly store = new Map<string, AuthorizationRequest>()

  async find(id: string): Promise<AuthorizationRequest | null> {
    const v = this.store.get(id)
    return v ? { ...v } : null
  }

  async save(req: AuthorizationRequest): Promise<void> {
    this.store.set(req.id, { ...req })
  }
}

export class InMemoryAuthorizationCodeStore implements AuthorizationCodeStore {
  private readonly store = new Map<string, AuthorizationCode>()

  async find(code: string): Promise<AuthorizationCode | null> {
    const v = this.store.get(code)
    return v ? { ...v } : null
  }

  async save(code: AuthorizationCode): Promise<void> {
    this.store.set(code.code, { ...code })
  }

  async redeem(code: string, now: Date = new Date()): Promise<AuthorizationCode | null> {
    const v = this.store.get(code)
    if (!v) return null
    if (v.redeemed_at) return null // 並行リプレイ
    if (Date.parse(v.expires_at) <= now.getTime()) return null
    const redeemed = { ...v, redeemed_at: now.toISOString() }
    this.store.set(code, redeemed)
    return redeemed
  }

  async linkFamily(code: string, family_id: string): Promise<void> {
    const v = this.store.get(code)
    if (!v) return
    this.store.set(code, { ...v, issued_family_id: family_id })
  }
}

export class InMemoryPARStore implements PARStore {
  private readonly store = new Map<string, PARRecord>()

  async find(request_uri: string): Promise<PARRecord | null> {
    const v = this.store.get(request_uri)
    return v ? { ...v } : null
  }

  async save(record: PARRecord): Promise<void> {
    this.store.set(record.request_uri, { ...record })
  }

  async consume(request_uri: string): Promise<PARRecord | null> {
    const v = this.store.get(request_uri)
    if (!v) return null
    if (v.used) return null
    if (Date.parse(v.expires_at) <= Date.now()) return null
    const used = { ...v, used: true }
    this.store.set(request_uri, used)
    return used
  }
}
