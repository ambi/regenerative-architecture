/**
 * Layer 4 — Adapter Layer（HTTP: /userinfo）
 *
 * OIDC Core §5.3.
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { oauthErrorResponse } from './error-response'
import { userInfoUseCase } from '../../src/oauth2/usecases/userinfo'
import type { TokenIntrospector } from '../../src/oauth2/ports/token-introspector'
import type { UserRepository } from '../../src/authentication/ports/user-repository'

export interface UserInfoRoutesDeps {
  introspector: TokenIntrospector
  userRepo: UserRepository
}

export function createUserInfoRoutes(deps: UserInfoRoutesDeps) {
  const app = new Hono()

  const handler = async (c: import('hono').Context) => {
    try {
      const authHeader = c.req.header('Authorization')
      if (!authHeader?.startsWith('Bearer ') && !authHeader?.startsWith('DPoP ')) {
        c.header('WWW-Authenticate', 'Bearer, DPoP')
        throw new OAuthError('invalid_request', 'Authorization ヘッダーが必要です')
      }
      const token = authHeader.split(' ')[1]
      const introspection = await deps.introspector.introspectAccessToken(token)
      if (!introspection.active || !introspection.sub || !introspection.client_id) {
        c.header('WWW-Authenticate', 'Bearer error="invalid_token"')
        throw new OAuthError('invalid_grant', 'トークンが無効です')
      }
      const scopes = introspection.scope?.split(/\s+/).filter(Boolean) ?? []
      const res = await userInfoUseCase(
        { userRepo: deps.userRepo },
        {
          scopes,
          sub: introspection.sub,
          active: introspection.active,
          client_id: introspection.client_id,
        },
      )
      return c.json(res)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  }

  app.get('/userinfo', handler)
  app.post('/userinfo', handler)
  return app
}
