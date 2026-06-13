/**
 * Layer 4 — Adapter Layer (Noop EmailSender)
 *
 * テスト用の no-op adapter。送信成功を返すが副作用は無い (ADR-030)。
 */

import type { EmailMessage, EmailSender } from '../../src/authentication/ports/email-sender'

export class NoopEmailSender implements EmailSender {
  readonly sent: EmailMessage[] = []

  async sendEmail(message: EmailMessage): Promise<boolean> {
    this.sent.push(message)
    return true
  }
}
