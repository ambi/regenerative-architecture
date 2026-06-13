/**
 * Layer 4 — Adapter Layer (HTTP: /forgot_password / /reset_password)
 *
 * パスワードリセットの未認証 HTTP 境界 (ADR-030)。
 *
 * - GET  /forgot_password           : SPA shell + CSRF cookie 発行 (未認証)
 * - POST /api/auth/forgot_password  : email を受け取り常に 204 (anti-enumeration)
 * - GET  /reset_password?token=...  : SPA shell + CSRF cookie (token は URL クエリ)
 * - POST /api/auth/reset_password   : token + new_password を消費して password 更新
 */

import { Hono } from 'hono'
import type { BreachedPasswordChecker } from '../../src/authentication/ports/breached-password-checker'
import type { EmailSender } from '../../src/authentication/ports/email-sender'
import type { PasswordHasher } from '../../src/authentication/ports/password-hasher'
import type { PasswordHistoryRepository } from '../../src/authentication/ports/password-history-repository'
import type { PasswordResetTokenStore } from '../../src/authentication/ports/password-reset-token-store'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import { requestPasswordReset } from '../../src/authentication/usecases/request-password-reset'
import {
  InvalidResetTokenError,
  PasswordReuseError,
  resetPasswordWithToken,
} from '../../src/authentication/usecases/reset-password-with-token'
import { PasswordPolicyError } from '../../src/authentication/usecases/password-policy'
import { requestTenantId } from './middleware/tenant-middleware'
import {
  assertCsrf,
  createCsrfToken,
  csrfCookie,
  WebSecurityError,
} from '../../src/shared/web-security'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import { noStoreJSON } from './browser-transaction'
import { renderShell } from './spa-shell'

export interface PasswordResetRoutesDeps {
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  passwordHistoryRepo: PasswordHistoryRepository
  passwordResetTokenStore: PasswordResetTokenStore
  emailSender: EmailSender
  breachedPasswordChecker: BreachedPasswordChecker
  emit: (e: DomainEvent) => void
  issuer: string
}

export function createPasswordResetRoutes(deps: PasswordResetRoutesDeps) {
  const app = new Hono()

  app.get('/forgot_password', (c) => {
    const csrf = createCsrfToken()
    const html = renderShell({
      page: 'forgot-password',
      title: 'パスワードのリセット',
      meta: { csrf },
      acceptLanguage: c.req.header('accept-language'),
    })
    return new Response(html, {
      status: 200,
      headers: {
        'content-type': 'text/html; charset=UTF-8',
        'set-cookie': csrfCookie(csrf),
      },
    })
  })

  app.post('/api/auth/forgot_password', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const body = await c.req.json().catch(() => null)
      const email = typeof body?.email === 'string' ? body.email : ''
      await requestPasswordReset(
        {
          userRepo: deps.userRepo,
          tokenStore: deps.passwordResetTokenStore,
          emailSender: deps.emailSender,
          emit: deps.emit,
          issuer: deps.issuer,
        },
        { tenant_id: requestTenantId(c), email },
      )
      // anti-enumeration: 受理を意味する 204 を常に返す。
      return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } })
    } catch (e) {
      if (e instanceof WebSecurityError) {
        return noStoreJSON(c, 403, { error: 'csrf_failed', message: e.message })
      }
      throw e
    }
  })

  app.get('/reset_password', (c) => {
    const csrf = createCsrfToken()
    const token = c.req.query('token') ?? ''
    const html = renderShell({
      page: 'reset-password',
      title: '新しいパスワードの設定',
      meta: { csrf, 'reset-token': token },
      acceptLanguage: c.req.header('accept-language'),
    })
    return new Response(html, {
      status: 200,
      headers: {
        'content-type': 'text/html; charset=UTF-8',
        'set-cookie': csrfCookie(csrf),
      },
    })
  })

  app.post('/api/auth/reset_password', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const body = await c.req.json().catch(() => null)
      const token = typeof body?.token === 'string' ? body.token : ''
      const newPassword = typeof body?.new_password === 'string' ? body.new_password : ''
      await resetPasswordWithToken(
        {
          userRepo: deps.userRepo,
          tokenStore: deps.passwordResetTokenStore,
          passwordHasher: deps.passwordHasher,
          historyRepo: deps.passwordHistoryRepo,
          breachedPasswordChecker: deps.breachedPasswordChecker,
          emit: deps.emit,
        },
        { token, new_password: newPassword },
      )
      return noStoreJSON(c, 200, { status: 'ok' })
    } catch (e) {
      if (e instanceof WebSecurityError) {
        return noStoreJSON(c, 403, { error: 'csrf_failed', message: e.message })
      }
      if (e instanceof InvalidResetTokenError) {
        return noStoreJSON(c, 410, {
          error: 'invalid_reset_token',
          message: 'リセットリンクが無効か期限切れです',
        })
      }
      if (e instanceof PasswordReuseError) {
        return noStoreJSON(c, 400, {
          error: 'password_reuse',
          message: '直近に使ったパスワードは再利用できません',
        })
      }
      if (e instanceof PasswordPolicyError) {
        return noStoreJSON(c, 400, {
          error: 'password_policy_violation',
          violations: e.violations,
        })
      }
      throw e
    }
  })

  return app
}
