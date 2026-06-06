/**
 * Layer 4 — Adapter Layer（HTTP: /introspect, /revoke）
 *
 * RFC 7662, RFC 7009.
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { authenticateClient } from './client-authentication'
import { oauthErrorResponse } from './error-response'
import { introspectTokenUseCase } from '../../src/oauth2/usecases/introspect-token'
import { revokeTokenUseCase } from '../../src/oauth2/usecases/revoke-token'
import type { AccessTokenDenylist } from '../../src/oauth2/ports/access-token-denylist'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { RefreshTokenStore } from '../../src/oauth2/ports/refresh-token-store'
import type { TokenIntrospector } from '../../src/oauth2/ports/token-introspector'
import type { ClientAssertionReplayStore } from '../../src/oauth2/ports/client-assertion-replay-store'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export interface IntrospectionRoutesDeps {
  issuer: string
  clientRepo: ClientRepository
  refreshStore: RefreshTokenStore
  introspector: TokenIntrospector
  accessTokenDenylist?: AccessTokenDenylist
  clientAssertionReplayStore: ClientAssertionReplayStore
  emit: (e: DomainEvent) => void
}

export function createIntrospectionRoutes(deps: IntrospectionRoutesDeps) {
  const app = new Hono()

  app.post('/introspect', async (c) => {
    try {
      const body = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
      const auth = await authenticateClient(c, body, deps.clientRepo, {
        issuer: deps.issuer,
        clientAssertionReplayStore: deps.clientAssertionReplayStore,
      })
      if (!body.token) throw new OAuthError('invalid_request', 'token が必要です')
      const res = await introspectTokenUseCase(
        {
          introspector: deps.introspector,
          refreshStore: deps.refreshStore,
          accessTokenDenylist: deps.accessTokenDenylist,
        },
        {
          token: body.token,
          token_type_hint: body.token_type_hint as 'access_token' | 'refresh_token' | undefined,
        },
      )
      deps.emit({
        type: 'TokenIntrospected',
        occurredAt: new Date().toISOString(),
        rsClientId: auth.client.client_id,
        tokenId: res.jti ?? 'unknown',
        active: res.active,
      })
      return c.json(res)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  app.post('/revoke', async (c) => {
    try {
      const body = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
      await authenticateClient(c, body, deps.clientRepo, {
        issuer: deps.issuer,
        clientAssertionReplayStore: deps.clientAssertionReplayStore,
      })
      if (!body.token) throw new OAuthError('invalid_request', 'token が必要です')
      await revokeTokenUseCase(
        {
          refreshStore: deps.refreshStore,
          introspector: deps.introspector,
          accessTokenDenylist: deps.accessTokenDenylist,
        },
        body.token,
        (e) => deps.emit(e as DomainEvent),
      )
      return c.body(null, 200)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}
