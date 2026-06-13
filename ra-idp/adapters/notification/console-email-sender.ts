/**
 * Layer 4 — Adapter Layer (Console EmailSender)
 *
 * stdout に email 本文を JSON 行で出力する開発・デモ用 adapter (ADR-030)。
 * 本番では SMTP / HTTP プロバイダ adapter に差し替える。
 *
 * fail-open: console.log 自体は失敗しないため例外経路は無いが、port 契約上
 * boolean を返す形に揃える。
 */

import type { EmailMessage, EmailSender } from '../../src/authentication/ports/email-sender'

export class ConsoleEmailSender implements EmailSender {
  async sendEmail(message: EmailMessage): Promise<boolean> {
    const payload = {
      kind: 'email',
      to: message.to,
      subject: message.subject,
      text: message.text,
      html_present: typeof message.html === 'string',
      sent_at: new Date().toISOString(),
    }
    console.log(JSON.stringify(payload))
    return true
  }
}
