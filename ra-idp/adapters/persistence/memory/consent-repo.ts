/**
 * Layer 4 — Adapter Layer (in-memory ConsentRepository)
 */

import type { Consent } from '../../../src/spec-bindings/schemas'
import type { ConsentRepository } from '../../../src/oauth2/ports/consent-repository'

export class InMemoryConsentRepository implements ConsentRepository {
  private readonly store = new Map<string, Consent>()

  private key(sub: string, client_id: string): string {
    return `${sub}::${client_id}`
  }

  async find(sub: string, client_id: string): Promise<Consent | null> {
    const c = this.store.get(this.key(sub, client_id))
    return c ? { ...c } : null
  }

  async save(consent: Consent): Promise<void> {
    this.store.set(this.key(consent.sub, consent.client_id), { ...consent })
  }

  async revoke(sub: string, client_id: string): Promise<void> {
    const k = this.key(sub, client_id)
    const existing = this.store.get(k)
    if (existing) {
      this.store.set(k, { ...existing, revoked_at: new Date().toISOString() })
    }
  }
}
