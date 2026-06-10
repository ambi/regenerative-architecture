/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * パスワード履歴の保管境界。ADR-027 で定義した PasswordHistoryNoReuse invariant の
 * adapter 接続点。エントリは `password_hash` と同じ PHC エンコード文字列を持ち、
 * 追加の暗号化は行わない（同じ攻撃耐性を持つため、二重管理しない）。
 */

export interface PasswordHistoryEntry {
  encoded: string
  created_at: string
}

export interface PasswordHistoryRepository {
  /**
   * sub の直近 depth 件を created_at DESC で返す。depth 0 以下は空配列。
   */
  recent(sub: string, depth: number): Promise<PasswordHistoryEntry[]>
  /**
   * sub の履歴に encoded を 1 件追加する。古いエントリの剪定は adapter 内に閉じても
   * usecase 側で行ってもよい（再利用チェックは常に depth 件のみを見る）。
   */
  add(sub: string, encoded: string, now: Date): Promise<void>
}
