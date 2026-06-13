/**
 * Layer 4 — Adapter Layer (in-memory ConsentRepository)
 */

import type { Consent } from '../../../src/spec-bindings/schemas'
import type { ConsentRepository } from '../../../src/oauth2/ports/consent-repository'

export class InMemoryConsentRepository implements ConsentRepository {
  private readonly store = new Map<string, Consent>()

  private key(tenant_id: string, sub: string, client_id: string): string {
    return `${tenant_id}::${sub}::${client_id}`
  }

  async find(tenant_id: string, sub: string, client_id: string): Promise<Consent | null> {
    const c = this.store.get(this.key(tenant_id, sub, client_id))
    return c ? { ...c } : null
  }

  async save(consent: Consent): Promise<void> {
    this.store.set(this.key(consent.tenant_id, consent.sub, consent.client_id), { ...consent })
  }

  async revoke(tenant_id: string, sub: string, client_id: string): Promise<void> {
    const k = this.key(tenant_id, sub, client_id)
    const existing = this.store.get(k)
    if (existing) {
      this.store.set(k, { ...existing, revoked_at: new Date().toISOString() })
    }
  }
}
