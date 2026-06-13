/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * クライアント永続化のインターフェース。実装は adapters/persistence/。
 */

import type { Client } from '../../spec-bindings/schemas'

export interface ClientRepository {
  findById(tenant_id: string, client_id: string): Promise<Client | null>
  save(client: Client): Promise<void>
  delete(tenant_id: string, client_id: string): Promise<void>
  findAll(tenant_id: string): Promise<Client[]>
}
