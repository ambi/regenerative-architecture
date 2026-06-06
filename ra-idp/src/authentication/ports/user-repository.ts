/**
 * Layer 3 — Application Logic（ポート定義）
 */

import type { User } from '../../spec-bindings/schemas'

export interface UserRepository {
  findBySub(sub: string): Promise<User | null>
  findByUsername(username: string): Promise<User | null>
  save(user: User): Promise<void>
}
