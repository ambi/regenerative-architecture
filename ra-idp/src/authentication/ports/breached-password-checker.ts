/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * 外部漏洩データベース検査の境界。bundled common-password 辞書（offline）が
 * 拾えない過去流出パスワードを弾くためのポート。詳細は ADR-028。
 *
 * adapter は HIBP k-anonymity API などを内部に閉じる。port 層は plain を受け、
 * 二値の breached 判定だけを返す。fail-open / 閾値判定は adapter 責務。
 */

export interface BreachedPasswordChecker {
  /**
   * plain が既知の漏洩データベースに含まれるかを判定する。
   * 外部 API 失敗時は実装側で false を返す（fail-open）。詳細は ADR-028。
   */
  isBreached(plain: string): Promise<boolean>
}
