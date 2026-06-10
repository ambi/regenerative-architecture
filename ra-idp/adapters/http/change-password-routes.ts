/**
 * Layer 4 — Adapter Layer (HTTP: /account/password)
 *
 * 認証済みユーザーが自身のパスワードを変更するための shell + JSON API。
 *
 * - GET  /account/password         : SPA shell + CSRF cookie 発行 (要 session)
 * - POST /api/auth/change_password : changePassword usecase を呼ぶ (要 session + CSRF)
 *
 * 認可コードフロー中 (browser transaction cookie あり) ではなく、既にログイン済の
 * ユーザーが行う設定画面なので、SPA shell は transaction cookie を持たない。
 * session cookie が無い場合は 401 を返し、SPA 側で /login への誘導を行う。
 */

import { Hono } from 'hono'
import type { SessionManager } from '../../src/authentication/usecases/session-manager'
import type { PasswordHasher } from '../../src/authentication/ports/password-hasher'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { PasswordHistoryRepository } from '../../src/authentication/ports/password-history-repository'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import {
  changePassword,
  CurrentPasswordMismatchError,
  PasswordReuseError,
  UserNotFoundError,
} from '../../src/authentication/usecases/change-password'
import { PasswordPolicyError } from '../../src/authentication/usecases/password-policy'
import {
  assertCsrf,
  createCsrfToken,
  csrfCookie,
  WebSecurityError,
} from '../../src/shared/web-security'
import { renderShell } from './spa-shell'
import { noStoreJSON } from './browser-transaction'

export interface ChangePasswordRoutesDeps {
  sessionManager: SessionManager
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  passwordHistoryRepo: PasswordHistoryRepository
  emit: (e: DomainEvent) => void
}

export function createChangePasswordRoutes(deps: ChangePasswordRoutesDeps) {
  const app = new Hono()

  app.get('/account/password', async (c) => {
    const context = await deps.sessionManager.resolve(c.req.raw.headers)
    if (!context || context.authentication_pending) {
      return new Response(null, { status: 303, headers: { location: '/login' } })
    }
    const csrf = createCsrfToken()
    const html = renderShell({
      page: 'change-password',
      title: 'パスワードの変更',
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

  app.post('/api/auth/change_password', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const context = await deps.sessionManager.resolve(c.req.raw.headers)
      if (!context || context.authentication_pending) {
        return noStoreJSON(c, 401, {
          error: 'session_required',
          message: 'ログインが必要です',
        })
      }
      const body = await c.req.json().catch(() => null)
      const current = typeof body?.current_password === 'string' ? body.current_password : ''
      const next = typeof body?.new_password === 'string' ? body.new_password : ''

      await changePassword(
        {
          userRepo: deps.userRepo,
          passwordHasher: deps.passwordHasher,
          historyRepo: deps.passwordHistoryRepo,
          emit: deps.emit,
        },
        { sub: context.sub, current_password: current, new_password: next },
      )
      return noStoreJSON(c, 200, { status: 'ok' })
    } catch (e) {
      if (e instanceof WebSecurityError) {
        return noStoreJSON(c, 403, { error: 'csrf_failed', message: e.message })
      }
      if (e instanceof CurrentPasswordMismatchError) {
        return noStoreJSON(c, 400, {
          error: 'current_password_mismatch',
          message: '現在のパスワードが一致しません',
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
      if (e instanceof UserNotFoundError) {
        return noStoreJSON(c, 401, { error: 'session_required', message: 'ログインが必要です' })
      }
      throw e
    }
  })

  return app
}
