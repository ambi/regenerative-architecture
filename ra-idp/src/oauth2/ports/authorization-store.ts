/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * 認可リクエスト・認可コード・PAR・デバイスコードの一時保存。
 * Redis などの揮発性ストレージを想定する。
 */

import type {
  AuthorizationRequest,
  AuthorizationCode,
  PARRecord,
} from '../../spec-bindings/schemas'

export interface AuthorizationRequestStore {
  find(id: string): Promise<AuthorizationRequest | null>
  save(req: AuthorizationRequest): Promise<void>
}

export interface AuthorizationCodeStore {
  find(code: string): Promise<AuthorizationCode | null>
  save(code: AuthorizationCode): Promise<void>
  /**
   * 認可コードを atomically 「redeemed」にする。
   * 既に redeemed なら null を返す（並行交換検出）。
   */
  redeem(code: string, now?: Date): Promise<AuthorizationCode | null>
  /**
   * 成功した交換で発行された refresh token のファミリー ID を紐付ける。
   * リプレイ検出時のファミリー失効に使う逆引きインデックス。
   */
  linkFamily(code: string, family_id: string): Promise<void>
}

export interface PARStore {
  find(request_uri: string): Promise<PARRecord | null>
  save(record: PARRecord): Promise<void>
  consume(request_uri: string): Promise<PARRecord | null>
}
