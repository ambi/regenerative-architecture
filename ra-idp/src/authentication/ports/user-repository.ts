/**
 * Layer 3 — Application Logic（ポート定義）
 */

import type { User } from '../../spec-bindings/schemas'

export interface UserRepository {
  findBySub(sub: string): Promise<User | null>
  findByUsername(tenant_id: string, username: string): Promise<User | null>
  /**
   * email アドレス (小文字正規化済み) で User を解決する。複数 user が同じ
   * email を持つことは登録時に禁ずる前提で、最初の 1 件 / null を返す。
   * email を持たない user は対象外。詳細は ADR-030。
   */
  findByEmail(tenant_id: string, email: string): Promise<User | null>
  /**
   * 全ユーザーを created_at 昇順で返す。管理 API のユーザー一覧 (ADR-031) で使う。
   * deleted_at が立った user は含めない。
   */
  findAll(tenant_id: string): Promise<User[]>
  save(user: User): Promise<void>
}
