/**
 * Layer 3 — Application Logic (request password reset use case)
 *
 * 未認証ユーザが email を入力して reset リンクの送信を要求する。
 * ADR-030: anti-enumeration のため戻り値は常に "受け付けた" の意。
 * verified email を持つ user が見つかったときだけ token を発行して送信する。
 *
 * フロー:
 *   1. PasswordResetRequested (emailHash) を emit (常に)
 *   2. user を email で検索
 *   3. user 不在 / email_verified=false → no-op で戻る
 *   4. 32 バイト乱数 token を生成、SHA-256 hash を store に保存
 *   5. EmailSender.sendEmail() で reset リンクを送信
 *   6. EmailSent (delivered, purpose=password_reset) を emit
 */

import { createHash, randomBytes } from 'crypto'
import type { DomainEvent } from '../../spec-bindings/schemas'
import type { EmailSender } from '../ports/email-sender'
import type { PasswordResetTokenStore } from '../ports/password-reset-token-store'
import type { UserRepository } from '../ports/user-repository'

export interface RequestPasswordResetDeps {
  userRepo: UserRepository
  tokenStore: PasswordResetTokenStore
  emailSender: EmailSender
  emit: (e: DomainEvent) => void
  /** reset URL を組み立てるための base (例: "https://idp.example.com")。 */
  issuer: string
  /** ADR-030 token TTL。省略時は 30 分。 */
  tokenTtlSeconds?: number
}

export interface RequestPasswordResetInput {
  email: string
  now?: Date
}

const DEFAULT_TTL_SECONDS = 1800

export async function requestPasswordReset(
  deps: RequestPasswordResetDeps,
  input: RequestPasswordResetInput,
): Promise<void> {
  const now = input.now ?? new Date()
  const emailLower = input.email.toLowerCase().trim()

  deps.emit({
    type: 'PasswordResetRequested',
    occurredAt: now.toISOString(),
    emailHash: sha256Hex(emailLower),
  })

  if (emailLower.length === 0) return

  const user = await deps.userRepo.findByEmail(emailLower)
  if (!user?.email_verified) return

  const rawToken = randomBytes(32).toString('base64url')
  const tokenHash = sha256Hex(rawToken)
  const ttlSeconds = deps.tokenTtlSeconds ?? DEFAULT_TTL_SECONDS
  const expiresAt = new Date(now.getTime() + ttlSeconds * 1000)

  await deps.tokenStore.save({
    sub: user.sub,
    token_hash: tokenHash,
    created_at: now.toISOString(),
    expires_at: expiresAt.toISOString(),
  })

  const resetUrl = buildResetUrl(deps.issuer, rawToken)
  const delivered = await deps.emailSender.sendEmail({
    to: emailLower,
    subject: 'Password reset',
    text: passwordResetText(resetUrl, ttlSeconds),
    html: passwordResetHtml(resetUrl, ttlSeconds),
  })

  deps.emit({
    type: 'EmailSent',
    occurredAt: now.toISOString(),
    toHash: sha256Hex(emailLower),
    purpose: 'password_reset',
    delivered,
  })
}

function sha256Hex(value: string): string {
  return createHash('sha256').update(value, 'utf8').digest('hex')
}

function buildResetUrl(issuer: string, rawToken: string): string {
  const base = issuer.endsWith('/') ? issuer.slice(0, -1) : issuer
  return `${base}/reset_password?token=${encodeURIComponent(rawToken)}`
}

function passwordResetText(resetUrl: string, ttlSeconds: number): string {
  const minutes = Math.round(ttlSeconds / 60)
  return [
    'A password reset was requested for your account.',
    '',
    `Open the link below within ${minutes} minutes to set a new password:`,
    resetUrl,
    '',
    'If you did not request this, you can safely ignore this email.',
  ].join('\n')
}

function passwordResetHtml(resetUrl: string, ttlSeconds: number): string {
  const minutes = Math.round(ttlSeconds / 60)
  const safeUrl = resetUrl.replaceAll('"', '&quot;')
  return [
    '<p>A password reset was requested for your account.</p>',
    `<p>Open the link below within ${minutes} minutes to set a new password:</p>`,
    `<p><a href="${safeUrl}">${safeUrl}</a></p>`,
    '<p>If you did not request this, you can safely ignore this email.</p>',
  ].join('')
}
