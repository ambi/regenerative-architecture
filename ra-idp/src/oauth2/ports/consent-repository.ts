/**
 * Layer 3 — Application Logic（ポート定義）
 */

import type { Consent } from '../../spec-bindings/schemas'

export interface ConsentRepository {
  find(sub: string, client_id: string): Promise<Consent | null>
  save(consent: Consent): Promise<void>
  revoke(sub: string, client_id: string): Promise<void>
}
