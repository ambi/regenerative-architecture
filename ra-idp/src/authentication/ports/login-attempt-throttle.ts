/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * ログイン試行のスロットリング境界。per-account / per-IP の二軸を独立に
 * カウントし、しきい値到達でロックを掛ける (ADR-029)。usecase 層は
 * (kind, key) を投げるだけで、ロック表現や TTL は adapter に閉じる。
 */

export type LoginThrottleKind = 'account' | 'ip'

export interface LoginThrottleAcquireResult {
  /** ロック中でなければ true、ロック中なら false。 */
  allowed: boolean
  /** allowed=false のとき、いつ再試行可能になるかの秒数（クライアントへの Retry-After 用）。 */
  retryAfterSeconds?: number
}

export interface LoginAttemptThrottle {
  /**
   * 現在ロック中かを判定する（カウントは進めない）。HTTP 入口で呼んで早期に
   * 429 を返すために使う。allowed=true なら認証ロジックを継続する。
   */
  tryAcquire(kind: LoginThrottleKind, key: string, now: Date): Promise<LoginThrottleAcquireResult>

  /**
   * 失敗を 1 件記録し、window 内のしきい値に達した場合は lockout を設定する。
   * しきい値到達で lockout を新規に設定したときに retryAfterSeconds を返す。
   * 戻り値の locked=true は「この呼び出しでロックに切り替わった」ことを表し、
   * SIEM へ送る LoginThrottled イベントの発火に使う。
   */
  recordFailure(
    kind: LoginThrottleKind,
    key: string,
    now: Date,
  ): Promise<{ locked: boolean; retryAfterSeconds?: number }>

  /**
   * 当該 kind/key のカウンタ / lockout を解除する。per-account は成功時にクリアし、
   * per-IP は ADR-029 の方針でクリアしない（呼ばない）。
   */
  recordSuccess(kind: LoginThrottleKind, key: string): Promise<void>
}
