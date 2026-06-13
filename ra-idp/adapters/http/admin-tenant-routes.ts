/**
 * Layer 4 — Adapter Layer (HTTP: /realms/default/admin/tenants)
 *
 * ADR-032 §6: SystemAdministrator (default control-plane tenant 所属 + role
 * `system_admin`) のみがテナント CRUD を実行できる。
 *
 * - GET    /admin/tenants                  : 一覧
 * - GET    /admin/tenants/:tenant_id       : 単体
 * - POST   /admin/tenants                  : 作成
 * - PATCH  /admin/tenants/:tenant_id       : display_name 更新
 * - POST   /admin/tenants/:tenant_id/disable : 無効化 (default 不可)
 * - POST   /admin/tenants/:tenant_id/enable  : 再有効化
 */

import { Hono } from 'hono'

import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { SessionManager } from '../../src/authentication/usecases/session-manager'
import type { TenantRepository } from '../../src/tenancy/ports/tenant-repository'
import {
  DefaultTenantImmutableError,
  DisplayNameRequiredError,
  InvalidTenantIdError,
  TenantConflictError,
  TenantNotFoundError,
  createTenant,
  setTenantDisabled,
  updateTenant,
} from '../../src/tenancy/usecases/manage-tenants'
import { DEFAULT_TENANT_ID, type DomainEvent } from '../../src/spec-bindings/schemas'
import { assertCsrf, WebSecurityError } from '../../src/shared/web-security'
import { noStoreJSON } from './browser-transaction'

export interface AdminTenantRoutesDeps {
  sessionManager: SessionManager
  userRepo: UserRepository
  tenantRepo: TenantRepository
  emit: (e: DomainEvent) => void
}

export function createAdminTenantRoutes(deps: AdminTenantRoutesDeps) {
  const app = new Hono()

  app.get('/admin/tenants', async (c) => {
    const actor = await resolveSystemAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'ok') return accessError(actor.kind)
    const tenants = await deps.tenantRepo.findAll()
    return noStoreJSON(c, 200, { tenants })
  })

  app.get('/admin/tenants/:tenant_id', async (c) => {
    const actor = await resolveSystemAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'ok') return accessError(actor.kind)
    const tenant = await deps.tenantRepo.findById(c.req.param('tenant_id'))
    if (!tenant) return noStoreJSON(c, 404, { error: 'tenant_not_found' })
    return noStoreJSON(c, 200, tenant)
  })

  app.post('/admin/tenants', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveSystemAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'ok') return accessError(actor.kind)
      const body = await c.req.json().catch(() => null)
      const id = typeof body?.id === 'string' ? body.id : ''
      const display_name = typeof body?.display_name === 'string' ? body.display_name : ''
      const now = new Date()
      const tenant = await createTenant(deps.tenantRepo, { id, display_name }, now)
      deps.emit({
        type: 'TenantCreated',
        occurredAt: now.toISOString(),
        actorSub: actor.sub,
        tenantId: tenant.id,
      })
      return noStoreJSON(c, 201, tenant)
    } catch (e) {
      return mapTenantError(e)
    }
  })

  app.patch('/admin/tenants/:tenant_id', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveSystemAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'ok') return accessError(actor.kind)
      const body = await c.req.json().catch(() => null)
      const display_name = typeof body?.display_name === 'string' ? body.display_name : ''
      const now = new Date()
      const tenant = await updateTenant(
        deps.tenantRepo,
        c.req.param('tenant_id'),
        { display_name },
        now,
      )
      deps.emit({
        type: 'TenantUpdated',
        occurredAt: now.toISOString(),
        actorSub: actor.sub,
        tenantId: tenant.id,
        changedFields: ['display_name'],
      })
      return noStoreJSON(c, 200, tenant)
    } catch (e) {
      return mapTenantError(e)
    }
  })

  app.post('/admin/tenants/:tenant_id/disable', async (c) =>
    handleSetDisabled(c, deps, c.req.param('tenant_id'), true),
  )
  app.post('/admin/tenants/:tenant_id/enable', async (c) =>
    handleSetDisabled(c, deps, c.req.param('tenant_id'), false),
  )

  return app
}

type SystemAdminResolution =
  | { kind: 'unauthorized' }
  | { kind: 'forbidden' }
  | { kind: 'ok'; sub: string }

async function resolveSystemAdmin(
  deps: AdminTenantRoutesDeps,
  headers: Headers,
): Promise<SystemAdminResolution> {
  const context = await deps.sessionManager.resolve(headers)
  if (!context || context.authentication_pending) return { kind: 'unauthorized' }
  const user = await deps.userRepo.findBySub(context.sub)
  if (!user || user.disabled_at) return { kind: 'forbidden' }
  if (user.tenant_id !== DEFAULT_TENANT_ID) return { kind: 'forbidden' }
  if (!user.roles.includes('system_admin')) return { kind: 'forbidden' }
  return { kind: 'ok', sub: user.sub }
}

async function handleSetDisabled(
  c: import('hono').Context,
  deps: AdminTenantRoutesDeps,
  tenantId: string,
  disabled: boolean,
): Promise<Response> {
  try {
    assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
    const actor = await resolveSystemAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'ok') return accessError(actor.kind)
    const now = new Date()
    const tenant = await setTenantDisabled(deps.tenantRepo, tenantId, disabled, now)
    deps.emit({
      type: disabled ? 'TenantDisabled' : 'TenantEnabled',
      occurredAt: now.toISOString(),
      actorSub: actor.sub,
      tenantId: tenant.id,
    })
    return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } })
  } catch (e) {
    return mapTenantError(e)
  }
}

function accessError(kind: 'unauthorized' | 'forbidden'): Response {
  const status = kind === 'unauthorized' ? 401 : 403
  const error = kind === 'unauthorized' ? 'authentication_required' : 'access_denied'
  return new Response(JSON.stringify({ error }), {
    status,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
}

function mapTenantError(e: unknown): Response {
  if (e instanceof WebSecurityError) {
    return jsonError(403, 'csrf_failed', e.message)
  }
  if (e instanceof TenantNotFoundError) {
    return jsonError(404, 'tenant_not_found', 'テナントが存在しません')
  }
  if (e instanceof TenantConflictError) {
    return jsonError(409, 'tenant_conflict', 'テナントIDは既に使用されています')
  }
  if (e instanceof InvalidTenantIdError) {
    return jsonError(400, 'invalid_request', 'tenant_id が不正です')
  }
  if (e instanceof DisplayNameRequiredError) {
    return jsonError(400, 'invalid_request', 'display_name が必要です')
  }
  if (e instanceof DefaultTenantImmutableError) {
    return jsonError(400, 'invalid_request', 'default テナントは無効化できません')
  }
  throw e
}

function jsonError(status: number, error: string, message: string): Response {
  return new Response(JSON.stringify({ error, message }), {
    status,
    headers: {
      'content-type': 'application/json; charset=UTF-8',
      'cache-control': 'no-store',
    },
  })
}
