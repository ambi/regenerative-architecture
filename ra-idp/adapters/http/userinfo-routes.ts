/**
 * Layer 4 — Adapter Layer（HTTP: /userinfo）
 *
 * OIDC Core §5.3 + RFC 9449 §7.1（DPoP-bound AT の resource access）。
 */

import { createHash } from 'crypto'
import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { oauthErrorResponse } from './error-response'
import { userInfoUseCase } from '../../src/oauth2/usecases/userinfo'
import { verifyDpopProof } from '../crypto/dpop-verifier'
import { parseClientCertificateHeader } from '../crypto/mtls-client-cert'
import { CLIENT_CERT_HEADER } from './client-authentication'
import type { AccessTokenDenylist } from '../../src/oauth2/ports/access-token-denylist'
import type { DpopNonceService } from '../../src/oauth2/ports/dpop-nonce-service'
import type { DpopReplayStore } from '../../src/oauth2/ports/dpop-replay-store'
import type { TokenIntrospector } from '../../src/oauth2/ports/token-introspector'
import type { UserRepository } from '../../src/authentication/ports/user-repository'

export interface UserInfoRoutesDeps {
  issuer: string
  introspector: TokenIntrospector
  userRepo: UserRepository
  dpopReplayStore: DpopReplayStore
  dpopNonceService: DpopNonceService
  accessTokenDenylist?: AccessTokenDenylist
}

export function createUserInfoRoutes(deps: UserInfoRoutesDeps) {
  const app = new Hono()
  const resourceUri = `${deps.issuer.replace(/\/$/, '')}/userinfo`

  const handler = async (c: import('hono').Context) => {
    c.header('DPoP-Nonce', deps.dpopNonceService.issue())
    try {
      const authHeader = c.req.header('Authorization')
      if (!authHeader?.startsWith('Bearer ') && !authHeader?.startsWith('DPoP ')) {
        c.header('WWW-Authenticate', 'Bearer, DPoP')
        throw new OAuthError('invalid_request', 'Authorization ヘッダーが必要です')
      }
      const scheme = authHeader.startsWith('DPoP ') ? 'DPoP' : 'Bearer'
      const token = authHeader.split(' ')[1]
      const introspection = await deps.introspector.introspectAccessToken(token)
      const revoked =
        introspection.active &&
        introspection.jti &&
        deps.accessTokenDenylist &&
        (await deps.accessTokenDenylist.isRevoked(introspection.jti))
      if (!introspection.active || revoked || !introspection.sub || !introspection.client_id) {
        c.header('WWW-Authenticate', `${scheme} error="invalid_token"`)
        throw new OAuthError('invalid_grant', 'トークンが無効です')
      }

      // RFC 9449 §7.1: DPoP-bound AT は DPoP scheme + 各リクエストの proof が必須
      const boundJkt = introspection.cnf?.jkt
      if (boundJkt) {
        if (scheme !== 'DPoP') {
          c.header('WWW-Authenticate', 'DPoP error="invalid_token"')
          throw new OAuthError(
            'invalid_grant',
            'DPoP-bound token は DPoP scheme で送信してください',
          )
        }
        const expectedAth = createHash('sha256').update(token).digest('base64url')
        const proof = await verifyDpopProof(c.req.header('DPoP'), c.req.method, resourceUri, {
          replayStore: deps.dpopReplayStore,
          expectedAth,
          nonceService: deps.dpopNonceService,
        })
        if (!proof || proof.jkt !== boundJkt) {
          c.header('WWW-Authenticate', 'DPoP error="invalid_token"')
          throw new OAuthError('invalid_grant', 'DPoP 鍵バインドが一致しません')
        }
      }

      // RFC 8705 §3: mTLS 証明書バインド AT は同じ証明書での提示が必須
      const boundCertThumbprint = introspection.cnf?.['x5t#S256']
      if (boundCertThumbprint) {
        const certHeader = c.req.header(CLIENT_CERT_HEADER)
        const cert = certHeader ? parseClientCertificateHeader(certHeader) : null
        if (!cert || cert.thumbprintS256 !== boundCertThumbprint) {
          c.header('WWW-Authenticate', `${scheme} error="invalid_token"`)
          throw new OAuthError('invalid_grant', 'mTLS 証明書バインドが一致しません')
        }
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
