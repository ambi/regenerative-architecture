/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * クライアント永続化のインターフェース。実装は adapters/persistence/。
 */

import type { Client } from '../spec-bindings/schemas'

export interface ClientRepository {
  findById(client_id: string): Promise<Client | null>
  save(client: Client): Promise<void>
  delete(client_id: string): Promise<void>
  findAll(): Promise<Client[]>
}
