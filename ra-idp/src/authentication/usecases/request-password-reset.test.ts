/**
 * Layer 3 — Application Logic (request-password-reset テスト)
 */

import { describe, expect, it } from 'bun:test'
import { createHash } from 'crypto'

import { InMemoryPasswordResetTokenStore } from '../../../adapters/persistence/memory/password-reset-token-store'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { NoopEmailSender } from '../../../adapters/notification/noop-email-sender'
import { UserSchema, type DomainEvent } from '../../spec-bindings/schemas'
import { requestPasswordReset } from './request-password-reset'

const ISSUER = 'http://idp.test'

async function setupUser(overrides: { email?: string; emailVerified?: boolean } = {}) {
  const userRepo = new InMemoryUserRepository()
  const tokenStore = new InMemoryPasswordResetTokenStore()
  const emailSender = new NoopEmailSender()
  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)
  await userRepo.save(
    UserSchema.parse({
      sub: 'user-alice',
      tenant_id: 'default',
      preferred_username: 'alice',
      password_hash: 'unused',
      email: overrides.email ?? 'alice@example.com',
      email_verified: overrides.emailVerified ?? true,
      mfa_enrolled: false,
      created_at: '2024-01-01T00:00:00.000Z',
      updated_at: '2024-01-01T00:00:00.000Z',
    }),
  )
  return { userRepo, tokenStore, emailSender, events, emit }
}

describe('requestPasswordReset', () => {
  it('verified email を持つ user に reset link を送信し PasswordResetRequested + EmailSent を emit する', async () => {
    const h = await setupUser()
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'alice@example.com', now: new Date('2026-06-13T12:00:00Z') },
    )
    expect(h.emailSender.sent).toHaveLength(1)
    expect(h.emailSender.sent[0].to).toBe('alice@example.com')
    expect(h.emailSender.sent[0].text).toContain(`${ISSUER}/reset_password?token=`)
    expect(h.events.map((e) => e.type)).toEqual(['PasswordResetRequested', 'EmailSent'])
  })

  it('未登録 email でも PasswordResetRequested を emit し送信せず正常終了 (anti-enumeration)', async () => {
    const h = await setupUser()
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'unknown@example.com' },
    )
    expect(h.emailSender.sent).toHaveLength(0)
    expect(h.events).toEqual([expect.objectContaining({ type: 'PasswordResetRequested' })])
  })

  it('email_verified=false の user には送信しない', async () => {
    const h = await setupUser({ emailVerified: false })
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'alice@example.com' },
    )
    expect(h.emailSender.sent).toHaveLength(0)
  })

  it('email は大文字小文字を正規化して解決される', async () => {
    const h = await setupUser({ email: 'alice@example.com' })
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'ALICE@Example.COM' },
    )
    expect(h.emailSender.sent).toHaveLength(1)
  })

  it('emailHash は SHA-256 (lowercased email) と一致する', async () => {
    const h = await setupUser()
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'ALICE@example.com' },
    )
    const requested = h.events.find((e) => e.type === 'PasswordResetRequested')
    expect(requested).toBeDefined()
    if (requested?.type === 'PasswordResetRequested') {
      expect(requested.emailHash).toBe(
        createHash('sha256').update('alice@example.com', 'utf8').digest('hex'),
      )
    }
  })

  it('発行された token は store に SHA-256 hash で保管される (生 token は持たない)', async () => {
    const h = await setupUser()
    const issued = new Date('2026-06-13T12:00:00Z')
    await requestPasswordReset(
      { ...h, issuer: ISSUER },
      { tenant_id: 'default', email: 'alice@example.com', now: issued },
    )
    const sentUrl = h.emailSender.sent[0].text
    const tokenMatch = sentUrl.match(/token=([A-Za-z0-9_-]+)/)
    expect(tokenMatch).not.toBeNull()
    const rawToken = decodeURIComponent(tokenMatch?.[1] ?? '')
    const expectedHash = createHash('sha256').update(rawToken, 'utf8').digest('hex')
    const record = await h.tokenStore.consume(expectedHash, new Date(issued.getTime() + 60_000))
    expect(record?.sub).toBe('user-alice')
  })
})
