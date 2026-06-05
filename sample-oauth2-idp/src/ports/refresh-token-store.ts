/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * リフレッシュトークンのストア。
 * ADR-004 のファミリー失効をサポートする。
 */

import type { RefreshTokenRecord } from '../spec-bindings/schemas'

export interface RefreshTokenStore {
  findByHash(hash: string): Promise<RefreshTokenRecord | null>
  save(record: RefreshTokenRecord): Promise<void>
  /**
   * トークンを「rotated」にマークし、新しいトークンレコードと不可分に保存する。
   * 並行ローテーションを防ぐため atomic 操作を期待する。
   */
  rotate(parentId: string, newRecord: RefreshTokenRecord): Promise<RefreshTokenRecord | null>
  /**
   * 指定 family_id のすべてのトークンを revoked にする。
   */
  revokeFamily(family_id: string): Promise<void>
}
