/**
 * Layer 4 — Adapter Layer（HTTP: /register）
 *
 * Dynamic Client Registration (RFC 7591) — 本サンプルでは認証なしで開放しているが、
 * 本番では Initial Access Token / Software Statement で保護する必要がある。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/domain/errors'
import { oauthErrorResponse } from './error-response'
import { registerClientUseCase } from '../../src/usecases/register-client'
import type { ClientRepository } from '../../src/ports/client-repository'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export interface RegistrationRoutesDeps {
  clientRepo: ClientRepository
  emit: (e: DomainEvent) => void
}

export function createRegistrationRoutes(deps: RegistrationRoutesDeps) {
  const app = new Hono()

  app.post('/register', async (c) => {
    try {
      const body = await c.req.json().catch(() => null)
      if (!body || typeof body !== 'object') {
        throw new OAuthError('invalid_request', 'JSON ボディが必要です')
      }
      const input = body as Record<string, unknown>
      const res = await registerClientUseCase(
        { clientRepo: deps.clientRepo },
        {
          client_name: input.client_name as string | undefined,
          client_type: (input.client_type as 'public' | 'confidential') ?? 'confidential',
          redirect_uris: (input.redirect_uris as string[]) ?? [],
          grant_types: input.grant_types as never,
          response_types: input.response_types as never,
          token_endpoint_auth_method: input.token_endpoint_auth_method as never,
          jwks: input.jwks as Record<string, unknown> | undefined,
          jwks_uri: input.jwks_uri as string | undefined,
          require_pushed_authorization_requests: input.require_pushed_authorization_requests as
            | boolean
            | undefined,
          dpop_bound_access_tokens: input.dpop_bound_access_tokens as boolean | undefined,
          scope: input.scope as string | undefined,
          fapi_profile: input.fapi_profile as never,
        },
      )
      deps.emit({
        type: 'ClientRegistered',
        occurredAt: new Date().toISOString(),
        clientId: res.client.client_id,
        clientType: res.client.client_type,
      })
      // RFC 7591 にあわせて、平文 client_secret は登録応答にのみ返す
      const out: Record<string, unknown> = { ...res.client }
      delete out.client_secret_hash
      if (res.client_secret) out.client_secret = res.client_secret
      return c.json(out, 201)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}
