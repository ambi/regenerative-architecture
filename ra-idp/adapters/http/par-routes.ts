/**
 * Layer 4 — Adapter Layer（HTTP: /par）
 *
 * Pushed Authorization Requests (RFC 9126)。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { authenticateClient } from './client-authentication'
import { oauthErrorResponse } from './error-response'
import { pushAuthorizationRequestUseCase } from '../../src/oauth2/usecases/push-authorization-request'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { PARStore } from '../../src/oauth2/ports/authorization-store'
import type { ClientAssertionReplayStore } from '../../src/oauth2/ports/client-assertion-replay-store'
import type { DomainEvent } from '../../src/spec-bindings/schemas'

export interface PARRoutesDeps {
  issuer: string
  clientRepo: ClientRepository
  parStore: PARStore
  clientAssertionReplayStore: ClientAssertionReplayStore
  emit: (e: DomainEvent) => void
}

export function createPARRoutes(deps: PARRoutesDeps) {
  const app = new Hono()

  app.post('/par', async (c) => {
    try {
      const body = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
      const auth = await authenticateClient(c, body, deps.clientRepo, {
        issuer: deps.issuer,
        clientAssertionReplayStore: deps.clientAssertionReplayStore,
      })
      const { client_id, client_secret, client_assertion, client_assertion_type, ...parameters } =
        body
      void client_id
      void client_secret
      void client_assertion
      void client_assertion_type
      const res = await pushAuthorizationRequestUseCase(
        { clientRepo: deps.clientRepo, parStore: deps.parStore },
        { client_id: auth.client.client_id, parameters: parameters as Record<string, string> },
      )
      deps.emit({
        type: 'PARStored',
        occurredAt: new Date().toISOString(),
        requestUri: res.request_uri,
        clientId: auth.client.client_id,
      })
      return c.json(res, 201)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}
