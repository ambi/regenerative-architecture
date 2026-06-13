/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * パスワードリセット用 token の保管境界 (ADR-030)。token は 32 バイト乱数 →
 * base64url で、保存時は SHA-256 hash のみを使う。adapter / DB に生 token は
 * 持たない。consume は原子的に行い、二重消費を防ぐ。
 */

export interface PasswordResetTokenRecord {
  sub: string
  token_hash: string
  created_at: string
  expires_at: string
}

export interface PasswordResetTokenStore {
  /**
   * record を保存する。同 sub の未消費 token は失効させる
   * (新しいリンクが来たら古いリンクは無効になる)。
   */
  save(record: PasswordResetTokenRecord): Promise<void>

  /**
   * token_hash で record を原子的に取り出し、同時に失効させる。
   * 期限切れ / 未存在 / 消費済みのいずれかなら null。
   */
  consume(tokenHash: string, now: Date): Promise<PasswordResetTokenRecord | null>
}
