/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * DPoP jti のリプレイ防止用に直近 10 分の jti を保持する。
 */

export interface DpopReplayStore {
  /**
   * jti が直近 windowSeconds 以内に観測されたかチェックし、
   * 未観測なら記録して true（新規）を返す。観測済みなら false。
   */
  recordIfNew(jti: string, windowSeconds: number, now?: Date): Promise<boolean>
}
