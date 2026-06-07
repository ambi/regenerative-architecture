/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * spec/scl.yaml MfaFactor 集合へのアクセス。identity は (sub, type) 複合キーで、
 * 1 ユーザに各 type 高々 1 個まで持てる。User.mfa_enrolled は本集合の非空性から
 * 導出する（sub に何らかの factor があれば true）。
 */

import type { MfaFactor, MfaFactorType } from '../../spec-bindings/schemas'

export interface MfaFactorRepository {
  /** 指定ユーザの全 factor を返す。順序は実装依存。 */
  listBySub(sub: string): Promise<MfaFactor[]>
  /** 指定 (sub, type) の factor を返す。存在しなければ null。 */
  find(sub: string, type: MfaFactorType): Promise<MfaFactor | null>
  /** factor を保存する。同じ (sub, type) があれば置き換える (upsert)。 */
  save(factor: MfaFactor): Promise<void>
  /** 指定 (sub, type) の factor を削除する。存在しなくてもエラーにしない。 */
  delete(sub: string, type: MfaFactorType): Promise<void>
}
