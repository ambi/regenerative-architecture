/**
 * Layer 4 — Adapter Layer (HTTP: /admin/clients)
 *
 * Mirrors ra-idp-go/internal/adapters/http/admin_client_handler.go.
 *
 * - GET    /admin/clients              : list (tenant 内)
 * - GET    /admin/clients/:client_id   : single
 * - POST   /admin/clients              : create
 * - PATCH  /admin/clients/:client_id   : update
 * - DELETE /admin/clients/:client_id   : delete
 *
 * 認可: admin role (per-tenant) かつ disabled_at == null。各テナントに対する
 * クライアント管理であり、SystemAdministrator (default のみ) と分離される。
 */

import { Hono } from 'hono'

import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { SessionManager } from '../../src/authentication/usecases/session-manager'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import {
  type AdminClientDeps,
  ClientNotFoundError,
  createAdminClient,
  deleteAdminClient,
  updateAdminClient,
} from '../../src/administration/usecases/clients'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import type { Client, DomainEvent, User } from '../../src/spec-bindings/schemas'
import { assertCsrf, WebSecurityError } from '../../src/shared/web-security'
import { noStoreJSON } from './browser-transaction'
import { requestTenantId } from './middleware/tenant-middleware'

export interface AdminClientRoutesDeps {
  sessionManager: SessionManager
  userRepo: UserRepository
  clientRepo: ClientRepository
  emit: (e: DomainEvent) => void
}

export function createAdminClientRoutes(deps: AdminClientRoutesDeps) {
  const app = new Hono()
  const usecaseDeps: AdminClientDeps = { clientRepo: deps.clientRepo, emit: deps.emit }

  app.get('/admin/clients', async (c) => {
    const actor = await resolveAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'admin') return adminAccessError(actor.kind)
    const tenantId = requestTenantId(c)
    if (actor.user.tenant_id !== tenantId) return adminAccessError('forbidden')
    const clients = await deps.clientRepo.findAll(tenantId)
    clients.sort((a, b) => (a.client_id < b.client_id ? -1 : a.client_id > b.client_id ? 1 : 0))
    return noStoreJSON(c, 200, { clients })
  })

  app.get('/admin/clients/:client_id', async (c) => {
    const actor = await resolveAdmin(deps, c.req.raw.headers)
    if (actor.kind !== 'admin') return adminAccessError(actor.kind)
    const tenantId = requestTenantId(c)
    if (actor.user.tenant_id !== tenantId) return adminAccessError('forbidden')
    const client = await deps.clientRepo.findById(tenantId, c.req.param('client_id'))
    if (!client) return jsonError(404, 'client_not_found', 'クライアントが存在しません')
    return noStoreJSON(c, 200, client)
  })

  app.post('/admin/clients', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      const tenantId = requestTenantId(c)
      if (actor.user.tenant_id !== tenantId) return adminAccessError('forbidden')
      const body = (await c.req.json().catch(() => null)) as Record<string, unknown> | null
      if (!body) return jsonError(400, 'invalid_request', 'JSONリクエストが不正です')
      const result = await createAdminClient(usecaseDeps, {
        actorSub: actor.user.sub,
        registration: {
          tenant_id: tenantId,
          client_name: body.client_name as string | undefined,
          client_type: (body.client_type as 'public' | 'confidential') ?? 'confidential',
          redirect_uris: (body.redirect_uris as string[]) ?? [],
          grant_types: body.grant_types as Client['grant_types'],
          response_types: body.response_types as Client['response_types'],
          token_endpoint_auth_method: body.token_endpoint_auth_method as
            | Client['token_endpoint_auth_method']
            | undefined,
          jwks: body.jwks as Record<string, unknown> | undefined,
          jwks_uri: body.jwks_uri as string | undefined,
          require_pushed_authorization_requests:
            body.require_pushed_authorization_requests as boolean | undefined,
          dpop_bound_access_tokens: body.dpop_bound_access_tokens as boolean | undefined,
          scope: body.scope as string | undefined,
          fapi_profile: body.fapi_profile as Client['fapi_profile'] | undefined,
        },
      })
      const out: Record<string, unknown> = { client: result.client }
      if (result.client_secret) out.client_secret = result.client_secret
      return noStoreJSON(c, 201, out)
    } catch (e) {
      return mapAdminClientError(e)
    }
  })

  app.patch('/admin/clients/:client_id', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      const tenantId = requestTenantId(c)
      if (actor.user.tenant_id !== tenantId) return adminAccessError('forbidden')
      const body = (await c.req.json().catch(() => null)) as Record<string, unknown> | null
      if (!body) return jsonError(400, 'invalid_request', 'JSONリクエストが不正です')
      const updated = await updateAdminClient(usecaseDeps, {
        actorSub: actor.user.sub,
        tenant_id: tenantId,
        client_id: c.req.param('client_id'),
        client_name: body.client_name as string | null | undefined,
        redirect_uris: body.redirect_uris as string[] | undefined,
        grant_types: body.grant_types as Client['grant_types'] | undefined,
        response_types: body.response_types as Client['response_types'] | undefined,
        scope: body.scope as string | undefined,
        require_pushed_authorization_requests:
          body.require_pushed_authorization_requests as boolean | undefined,
        dpop_bound_access_tokens: body.dpop_bound_access_tokens as boolean | undefined,
      })
      return noStoreJSON(c, 200, updated)
    } catch (e) {
      return mapAdminClientError(e)
    }
  })

  app.delete('/admin/clients/:client_id', async (c) => {
    try {
      assertCsrf(c.req.header('Cookie'), c.req.header('X-CSRF-Token') ?? '')
      const actor = await resolveAdmin(deps, c.req.raw.headers)
      if (actor.kind !== 'admin') return adminAccessError(actor.kind)
      const tenantId = requestTenantId(c)
      if (actor.user.tenant_id !== tenantId) return adminAccessError('forbidden')
      await deleteAdminClient(usecaseDeps, {
        actorSub: actor.user.sub,
        tenant_id: tenantId,
        client_id: c.req.param('client_id'),
      })
      return new Response(null, { status: 204, headers: { 'cache-control': 'no-store' } })
    } catch (e) {
      return mapAdminClientError(e)
    }
  })

  return app
}

type AdminResolution =
  | { kind: 'unauthorized' }
  | { kind: 'forbidden' }
  | { kind: 'admin'; user: User }

async function resolveAdmin(
  deps: AdminClientRoutesDeps,
  headers: Headers,
): Promise<AdminResolution> {
  const ctx = await deps.sessionManager.resolve(headers)
  if (!ctx || ctx.authentication_pending) return { kind: 'unauthorized' }
  const user = await deps.userRepo.findBySub(ctx.sub)
  if (!user || user.disabled_at) return { kind: 'forbidden' }
  if (!user.roles.includes('admin')) return { kind: 'forbidden' }
  return { kind: 'admin', user }
}

function adminAccessError(kind: 'unauthorized' | 'forbidden'): Response {
  const status = kind === 'unauthorized' ? 401 : 403
  const error = kind === 'unauthorized' ? 'authentication_required' : 'access_denied'
  return jsonError(status, error, kind === 'unauthorized' ? '認証済みセッションが必要です' : '管理者権限が必要です')
}

function mapAdminClientError(e: unknown): Response {
  if (e instanceof WebSecurityError) {
    return jsonError(403, 'csrf_failed', e.message)
  }
  if (e instanceof ClientNotFoundError) {
    return jsonError(404, 'client_not_found', 'クライアントが存在しません')
  }
  if (e instanceof OAuthError) {
    return jsonError(400, e.code, e.message)
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

