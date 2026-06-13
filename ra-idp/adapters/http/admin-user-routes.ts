/**
 * Layer 4 — Adapter Layer (HTTP: /admin/users)
 *
 * 管理 API (Phase 4 / ADR-031)。認証済み browser session の sub から User を
 * 解決し、`admin` ロールかつ `disabled_at == null` の場合だけ通す。変更系は
 * CSRF と Origin 検証 (Hono の二重提出 CSRF) を加える。
 *
 * - GET    /admin/users               : SPA shell + CSRF cookie
 * - GET    /api/admin/users           : 一覧 JSON
 * - GET    /api/admin/users/:sub      : 個別 JSON
 * - POST   /api/admin/users           : 作成
 * - PATCH  /api/admin/users/:sub      : 更新
 * - POST   /api/admin/users/:sub/disable : 無効化
 * - POST   /api/admin/users/:sub/enable  : 再有効化
 */

import { Hono, type Context } from 'hono'

import type { PasswordHasher } from '../../src/authentication/ports/password-hasher'
import type { PasswordHistoryRepository } from '../../src/authentication/ports/password-history-repository'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { SessionManager } from '../../src/authentication/usecases/session-manager'
import {
  type AdminUserDeps,
  InvalidRoleError,
  UsernameConflictError,
  UserNotFoundError,
  createAdminUser,
  setAdminUserDisabled,
  updateAdminUser,
} from '../../src/administration/usecases/users'
import { PasswordPolicyError } from '../../src/authentication/usecases/password-policy'
import {
  AdminUserCreateRequestSchema,
  AdminUserUpdateRequestSchema,
  type AdminUserResponse,
  type DomainEvent,
  type User,
} from '../../src/spec-bindings/schemas'
import {
  assertCsrf,
  createCsrfToken,
  csrfCookie,
  WebSecurityError,
} from '../../src/shared/web-security'
import { noStoreJSON } from './browser-transaction'
import {
  requestTenantId,
  tenantCookiePath,
  tenantRoute,
  tenantUrlPrefix,
} from './middleware/tenant-middleware'
import { renderShell } from './spa-shell'

export interface AdminUserRoutesDeps {
  sessionManager: SessionManager
  userRepo: UserRepository
  passwordHasher: PasswordHasher
  passwordHistoryRepo: PasswordHistoryRepository
  emit: (e: DomainEvent) => void
}

export function createAdminUserRoutes(deps: AdminUserRoutesDeps) {
  const app = new Hono()
  const usecaseDeps: AdminUserDeps = {
    userRepo: deps.userRepo,
    passwordHasher: deps.passwordHasher,
    passwordHistoryRepo: deps.passwordHistoryRepo,
    emit: deps.emit,
  }

  app.get('/admin', (c) => c.redirect(tenantRoute(c, '/admin/users'), 303))

  app.get('/admin/users', async (c) => {
    const actor = await resolveAdmin(deps, c.req.raw.headers)
    if (actor.kind === 'unauthorized') return loginRedirect(c)
    if (actor.kind === 'forbidden') {
      return forbiddenShell(c.req.header('accept-language'))
    }
    if (actor.user.tenant_id !== requestTenantId(c))
      return forbiddenShell(c.req.header('accept-language'))
    const csrf = createCsrfToken()
    const html = renderShell({
      page: 'admin-users',
      title: 'ユーザー管理',
      meta: {
        csrf,
        'base-path': tenantUrlPrefix(c),
        'actor-username': actor.user.preferred_username,
      },
      acceptLanguage: c.req.header('accept-language'),
      tenantId: requestTenantId(c),
    })
    return new Response(html, {
      status: 200,
      headers: {
        'content-type': 'text/html; charset=UTF-8',
        'set-cookie': csrfCookie(csrf, tenantCookiePath(c)),
      },
    })
  })

  app.get('/api/admin/users', async (c) => {
    const actor = await resolveAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'admin') return adminAccessError(actor.kind)
    if (actor.user.tenant_id !== requestTenantId(c)) return adminAccessError('forbidden')
    const users = await deps.userRepo.findAll(requestTenantId(c))
    return noStoreJSON(c, 200, { users: users.map(toAdminUserResponse) })
  })

  app.get('/api/admin/users/:sub', async (c) => {
    const actor = await resolveAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'admin') return adminAccessError(actor.kind)
    if (actor.user.tenant_id !== requestTenantId(c)) return adminAccessError('forbidden')
    const user = await deps.userRepo.findBySub(c.req.param('sub'))
    if (!user || user.tenant_id !== requestTenantId(c)) {
      return noStoreJSON(c, 404, { error: 'user_not_found', message: 'ユーザーが存在しません' })
    }
    return noStoreJSON(c, 200, toAdminUserResponse(user))
  })

  app.post('/api/admin/users', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      if (actor.user.tenant_id !== requestTenantId(c)) return adminAccessError('forbidden')
      const body = await c.req.json().catch(() => null)
      const parsed = AdminUserCreateRequestSchema.safeParse(body)
      if (!parsed.success) {
        return noStoreJSON(c, 400, {
          error: 'invalid_request',
          message: 'JSONリクエストが不正です',
        })
      }
      const user = await createAdminUser(usecaseDeps, {
        actorSub: actor.user.sub,
        tenant_id: requestTenantId(c),
        preferred_username: parsed.data.preferred_username,
        password: parsed.data.password,
        name: parsed.data.name,
        email: parsed.data.email,
        email_verified: parsed.data.email_verified,
        roles: parsed.data.roles,
      })
      return noStoreJSON(c, 201, toAdminUserResponse(user))
    } catch (e) {
      return mapAdminError(e)
    }
  })

  app.patch('/api/admin/users/:sub', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      if (actor.user.tenant_id !== requestTenantId(c)) return adminAccessError('forbidden')
      const body = await c.req.json().catch(() => null)
      const parsed = AdminUserUpdateRequestSchema.safeParse(body)
      if (!parsed.success) {
        return noStoreJSON(c, 400, {
          error: 'invalid_request',
          message: 'JSONリクエストが不正です',
        })
      }
      const user = await updateAdminUser(usecaseDeps, {
        actorSub: actor.user.sub,
        sub: c.req.param('sub'),
        preferred_username: parsed.data.preferred_username,
        name: parsed.data.name,
        email: parsed.data.email,
        email_verified: parsed.data.email_verified,
        roles: parsed.data.roles,
      })
      return noStoreJSON(c, 200, toAdminUserResponse(user))
    } catch (e) {
      return mapAdminError(e)
    }
  })

  app.post('/api/admin/users/:sub/disable', (c) => setDisabled(c, true))
  app.post('/api/admin/users/:sub/enable', (c) => setDisabled(c, false))

  async function setDisabled(c: Context, disabled: boolean): Promise<Response> {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      if (actor.user.tenant_id !== requestTenantId(c)) return adminAccessError('forbidden')
      await setAdminUserDisabled(usecaseDeps, {
        actorSub: actor.user.sub,
        sub: c.req.param('sub') ?? '',
        disabled,
      })
      return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } })
    } catch (e) {
      return mapAdminError(e)
    }
  }

  return app
}

type AdminResolution =
  | { kind: 'unauthorized' }
  | { kind: 'forbidden' }
  | { kind: 'admin'; user: User }

async function resolveAdmin(deps: AdminUserRoutesDeps, headers: Headers): Promise<AdminResolution> {
  const context = await deps.sessionManager.resolve(headers)
  if (!context || context.authentication_pending) return { kind: 'unauthorized' }
  const user = await deps.userRepo.findBySub(context.sub)
  if (!user || user.disabled_at) return { kind: 'forbidden' }
  if (!user.roles.includes('admin')) return { kind: 'forbidden' }
  return { kind: 'admin', user }
}

function adminAccessError(kind: 'unauthorized' | 'forbidden'): Response {
  if (kind === 'unauthorized') {
    return new Response(
      JSON.stringify({
        error: 'authentication_required',
        message: '認証済みセッションが必要です',
      }),
      {
        status: 401,
        headers: {
          'content-type': 'application/json; charset=UTF-8',
          'cache-control': 'no-store',
        },
      },
    )
  }
  return new Response(JSON.stringify({ error: 'access_denied', message: '管理者権限が必要です' }), {
    status: 403,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
}

function mapAdminError(e: unknown): Response {
  if (e instanceof WebSecurityError) {
    return new Response(JSON.stringify({ error: 'csrf_failed', message: e.message }), {
      status: 403,
      headers: { 'content-type': 'application/json; charset=UTF-8', 'cache-control': 'no-store' },
    })
  }
  if (e instanceof UserNotFoundError) {
    return new Response(
      JSON.stringify({ error: 'user_not_found', message: 'ユーザーが存在しません' }),
      {
        status: 404,
        headers: {
          'content-type': 'application/json; charset=UTF-8',
          'cache-control': 'no-store',
        },
      },
    )
  }
  if (e instanceof UsernameConflictError) {
    return new Response(
      JSON.stringify({ error: 'username_conflict', message: 'ユーザー名は既に使用されています' }),
      {
        status: 409,
        headers: {
          'content-type': 'application/json; charset=UTF-8',
          'cache-control': 'no-store',
        },
      },
    )
  }
  if (e instanceof InvalidRoleError) {
    return new Response(JSON.stringify({ error: 'invalid_role', message: 'roleが不正です' }), {
      status: 400,
      headers: {
        'content-type': 'application/json; charset=UTF-8',
        'cache-control': 'no-store',
      },
    })
  }
  if (e instanceof PasswordPolicyError) {
    return new Response(
      JSON.stringify({
        error: 'password_policy',
        message: 'パスワードがセキュリティ要件を満たしていません',
        violations: e.violations,
      }),
      {
        status: 400,
        headers: {
          'content-type': 'application/json; charset=UTF-8',
          'cache-control': 'no-store',
        },
      },
    )
  }
  throw e
}

function loginRedirect(c: Context): Response {
  const returnTo = tenantRoute(c, '/admin/users')
  return new Response(null, {
    status: 303,
    headers: { location: `${tenantRoute(c, '/login')}?return_to=${encodeURIComponent(returnTo)}` },
  })
}

function forbiddenShell(acceptLanguage?: string): Response {
  const html = renderShell({
    page: 'error',
    title: '管理者権限が必要です',
    meta: {
      'error-kind': 'access_denied',
      'error-title': '管理者権限が必要です',
      'error-description': '/admin/users を表示するには admin ロールが必要です。',
    },
    acceptLanguage,
  })
  return new Response(html, {
    status: 403,
    headers: {
      'content-type': 'text/html; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
}

function toAdminUserResponse(user: User): AdminUserResponse {
  return {
    sub: user.sub,
    tenant_id: user.tenant_id,
    preferred_username: user.preferred_username,
    name: user.name,
    email: user.email,
    email_verified: user.email_verified,
    mfa_enrolled: user.mfa_enrolled,
    roles: [...user.roles],
    disabled_at: user.disabled_at,
    created_at: user.created_at,
    updated_at: user.updated_at,
  }
}
