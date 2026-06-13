/**
 * Layer 4 — Adapter Layer（HTTP: /token）
 *
 * トークンエンドポイント。grant_type ごとに分岐する:
 *   - authorization_code → exchangeCodeForTokenUseCase
 *   - refresh_token      → refreshTokenUseCase
 *   - client_credentials → 直接 access_token 発行
 *   - device_code        → exchangeDeviceCodeUseCase (RFC 8628, ADR-025)
 *
 * クライアント認証はミドルウェアで先行実行。
 * DPoP ヘッダーがあれば検証してセンダー制約付きトークンを発行する。
 */

import { Hono } from 'hono'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { authenticateClient } from './client-authentication'
import { oauthErrorResponse } from './error-response'
import { exchangeCodeForTokenUseCase } from '../../src/oauth2/usecases/exchange-code-for-token'
import { refreshTokenUseCase } from '../../src/oauth2/usecases/refresh-tokens'
import { exchangeDeviceCodeUseCase } from '../../src/oauth2/usecases/exchange-device-code'
import { verifyDpopProof } from '../crypto/dpop-verifier'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import type { UserRepository } from '../../src/authentication/ports/user-repository'
import type { AuthorizationCodeStore } from '../../src/oauth2/ports/authorization-store'
import type { RefreshTokenStore } from '../../src/oauth2/ports/refresh-token-store'
import type { DeviceCodeStore } from '../../src/oauth2/ports/device-code-store'
import type { TokenIssuer } from '../../src/oauth2/ports/token-issuer'
import type { DpopNonceService } from '../../src/oauth2/ports/dpop-nonce-service'
import type { DpopReplayStore } from '../../src/oauth2/ports/dpop-replay-store'
import type { ClientAssertionReplayStore } from '../../src/oauth2/ports/client-assertion-replay-store'
import type { DomainEvent } from '../../src/spec-bindings/schemas'
import type { GrantType } from '../../src/spec-bindings/grants/grant-types'
import { SUPPORTED_GRANT_TYPES } from '../../src/spec-bindings/grants/grant-types'

export interface TokenRoutesDeps {
  issuer: string
  clientRepo: ClientRepository
  userRepo: UserRepository
  codeStore: AuthorizationCodeStore
  refreshStore: RefreshTokenStore
  deviceCodeStore: DeviceCodeStore
  tokenIssuer: TokenIssuer
  dpopReplayStore: DpopReplayStore
  dpopNonceService: DpopNonceService
  clientAssertionReplayStore: ClientAssertionReplayStore
  emit: (e: DomainEvent) => void
}

export function createTokenRoutes(deps: TokenRoutesDeps) {
  const app = new Hono()

  app.post('/token', async (c) => {
    // RFC 9449 §8: 任意のレスポンスで新 nonce を提示してローテーションする
    c.header('DPoP-Nonce', deps.dpopNonceService.issue())
    try {
      const body = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
      const auth = await authenticateClient(c, body, deps.clientRepo, {
        issuer: deps.issuer,
        clientAssertionReplayStore: deps.clientAssertionReplayStore,
      })

      const grantType = body.grant_type
      if (!grantType) throw new OAuthError('invalid_request', 'grant_type が必要です')
      if (!(SUPPORTED_GRANT_TYPES as readonly string[]).includes(grantType)) {
        throw new OAuthError('unsupported_grant_type', `未対応の grant_type: ${grantType}`)
      }
      if (!auth.client.grant_types.includes(grantType as GrantType)) {
        throw new OAuthError('unauthorized_client', '宣言外の grant_type です')
      }

      // DPoP ヘッダーがあれば検証 (RFC 9449 §8 nonce 必須)
      const dpop = c.req.header('DPoP')
      const tokenUrl = `${deps.issuer}/token`
      const dpopResult = dpop
        ? await verifyDpopProof(dpop, 'POST', tokenUrl, {
            replayStore: deps.dpopReplayStore,
            nonceService: deps.dpopNonceService,
          })
        : null

      if (grantType === 'authorization_code') {
        const { response, audit } = await exchangeCodeForTokenUseCase(
          {
            clientRepo: deps.clientRepo,
            userRepo: deps.userRepo,
            codeStore: deps.codeStore,
            refreshStore: deps.refreshStore,
            tokenIssuer: deps.tokenIssuer,
          },
          {
            tenant_id: auth.client.tenant_id,
            client_id: auth.client.client_id,
            code: body.code ?? '',
            code_verifier: body.code_verifier ?? '',
            redirect_uri: body.redirect_uri ?? '',
            dpop_jkt: dpopResult?.jkt,
            mtls_x5t_s256: auth.mtlsThumbprintS256,
          },
        )
        const occurredAt = new Date().toISOString()
        deps.emit({
          type: 'AuthorizationCodeRedeemed',
          occurredAt,
          clientId: auth.client.client_id,
          sub: audit.sub,
        })
        deps.emit({
          type: 'AccessTokenIssued',
          occurredAt,
          jti: audit.jti,
          clientId: auth.client.client_id,
          sub: audit.sub,
          scopes: audit.scopes,
          senderConstraint: audit.senderConstraint,
        })
        if (audit.refreshTokenId && audit.refreshFamilyId) {
          deps.emit({
            type: 'RefreshTokenIssued',
            occurredAt,
            tokenId: audit.refreshTokenId,
            familyId: audit.refreshFamilyId,
            clientId: auth.client.client_id,
            sub: audit.sub,
          })
        }
        return c.json(response)
      }

      if (grantType === 'refresh_token') {
        if (!body.refresh_token) {
          throw new OAuthError('invalid_request', 'refresh_token が必要です')
        }
        const res = await refreshTokenUseCase(
          {
            clientRepo: deps.clientRepo,
            userRepo: deps.userRepo,
            refreshStore: deps.refreshStore,
            tokenIssuer: deps.tokenIssuer,
          },
          {
            tenant_id: auth.client.tenant_id,
            client_id: auth.client.client_id,
            refresh_token: body.refresh_token,
            proof_jkt: dpopResult?.jkt,
            proof_x5t_s256: auth.mtlsThumbprintS256,
          },
          (e) => deps.emit(e as DomainEvent),
        )
        return c.json(res)
      }

      if (grantType === 'client_credentials') {
        if (auth.client.client_type !== 'confidential') {
          throw new OAuthError('unauthorized_client', 'public client は不可')
        }
        const scopes = (body.scope ?? auth.client.scope).split(/\s+/).filter(Boolean)
        // 宣言スコープの部分集合性チェック
        const declared = auth.client.scope.split(/\s+/).filter(Boolean)
        if (!scopes.every((s) => declared.includes(s))) {
          throw new OAuthError('invalid_scope', '宣言外のスコープが含まれます')
        }
        const ccSenderConstraint:
          | { type: 'dpop'; jkt: string }
          | { type: 'mtls'; 'x5t#S256': string }
          | null = dpopResult
          ? { type: 'dpop', jkt: dpopResult.jkt }
          : auth.mtlsThumbprintS256
            ? { type: 'mtls', 'x5t#S256': auth.mtlsThumbprintS256 }
            : null
        const { token, jti } = await deps.tokenIssuer.signAccessToken({
          client: auth.client,
          sub: auth.client.client_id, // self-issued
          scopes,
          senderConstraint: ccSenderConstraint,
          authTime: Math.floor(Date.now() / 1000),
        })
        deps.emit({
          type: 'AccessTokenIssued',
          occurredAt: new Date().toISOString(),
          jti,
          clientId: auth.client.client_id,
          sub: auth.client.client_id,
          scopes,
          senderConstraint: ccSenderConstraint?.type ?? 'none',
        })
        return c.json({
          access_token: token,
          // RFC 8705 §3.2: mTLS バインドでも token_type は Bearer。
          token_type: ccSenderConstraint?.type === 'dpop' ? 'DPoP' : 'Bearer',
          expires_in: deps.tokenIssuer.getAccessTokenTtlSeconds(),
          scope: scopes.join(' '),
        })
      }

      if (grantType === 'urn:ietf:params:oauth:grant-type:device_code') {
        if (!body.device_code) {
          throw new OAuthError('invalid_request', 'device_code が必要です')
        }
        const { response, audit } = await exchangeDeviceCodeUseCase(
          {
            clientRepo: deps.clientRepo,
            userRepo: deps.userRepo,
            deviceCodeStore: deps.deviceCodeStore,
            refreshStore: deps.refreshStore,
            tokenIssuer: deps.tokenIssuer,
          },
          {
            tenant_id: auth.client.tenant_id,
            client_id: auth.client.client_id,
            device_code: body.device_code,
            dpop_jkt: dpopResult?.jkt,
            mtls_x5t_s256: auth.mtlsThumbprintS256,
          },
        )
        const occurredAt = new Date().toISOString()
        deps.emit({
          type: 'AccessTokenIssued',
          occurredAt,
          jti: audit.jti,
          clientId: auth.client.client_id,
          sub: audit.sub,
          scopes: audit.scopes,
          senderConstraint: audit.senderConstraint,
        })
        deps.emit({
          type: 'RefreshTokenIssued',
          occurredAt,
          tokenId: audit.refreshTokenId,
          familyId: audit.refreshFamilyId,
          clientId: auth.client.client_id,
          sub: audit.sub,
        })
        return c.json(response)
      }

      throw new OAuthError('unsupported_grant_type', `未対応の grant_type: ${grantType}`)
    } catch (e) {
      if (e instanceof OAuthError) return oauthErrorResponse(c, e)
      throw e
    }
  })

  return app
}
