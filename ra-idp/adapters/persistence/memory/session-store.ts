/**
 * Layer 4 — Adapter Layer (in-memory SessionStore)
 */

import type { LoginSession, SessionStore } from '../../../src/authentication/ports/session-store'

export class InMemorySessionStore implements SessionStore {
  private readonly store = new Map<string, LoginSession>()

  async find(id: string): Promise<LoginSession | null> {
    const v = this.store.get(id)
    if (!v) return null
    if (Date.parse(v.expires_at) <= Date.now()) {
      this.store.delete(id)
      return null
    }
    return { ...v }
  }

  async save(session: LoginSession): Promise<void> {
    this.store.set(session.id, { ...session })
  }

  async delete(id: string): Promise<void> {
    this.store.delete(id)
  }
}
