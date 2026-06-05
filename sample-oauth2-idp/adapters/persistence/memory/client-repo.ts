/**
 * Layer 4 — Adapter Layer (in-memory ClientRepository)
 *
 * テスト・ローカル開発用。ADR-016 が選定した Postgres adapter と同じ契約 (ClientRepository) に
 * 従う。両者の挙動の同一性は src/spec-bindings/persistence-contract.test.ts で検証される。
 */

import type { Client } from '../../../src/spec-bindings/schemas'
import type { ClientRepository } from '../../../src/ports/client-repository'

export class InMemoryClientRepository implements ClientRepository {
  private readonly store = new Map<string, Client>()

  async findById(client_id: string): Promise<Client | null> {
    const c = this.store.get(client_id)
    return c ? { ...c } : null
  }

  async save(client: Client): Promise<void> {
    this.store.set(client.client_id, { ...client })
  }

  async delete(client_id: string): Promise<void> {
    this.store.delete(client_id)
  }

  async findAll(): Promise<Client[]> {
    return [...this.store.values()].map((c) => ({ ...c }))
  }
}
