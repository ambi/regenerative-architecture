/**
 * Layer 3 — Application Logic（ポート定義）
 *
 * 認証ドメインから外部へ送るメールの境界 (ADR-030)。adapter は SMTP /
 * Resend / SendGrid 等の差し替えポイントになる。
 *
 * 失敗時の取り扱いは adapter 責務 (fail-open: 例外を usecase に伝播させない)。
 * SIEM 経由で配送結果を追えるよう、配送結果は `EmailSent` event として
 * caller 側で記録する。
 */

export interface EmailMessage {
  to: string
  subject: string
  text: string
  html?: string
}

export interface EmailSender {
  /**
   * email を送信する。adapter は送信成否を boolean で返す
   * (例外は内部で握りつぶす)。caller は戻り値で EmailSent.delivered を埋める。
   */
  sendEmail(message: EmailMessage): Promise<boolean>
}
