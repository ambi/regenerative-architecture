/**
 * Layer 3 — Application Logic (cross-tenant isolation)
 *
 * Mirrors ra-idp-go/internal/oauth2/usecases/tenant_isolation_test.go.
 * ADR-034 のテナント境界を usecase で binding しているか確認する。
 * - authorize: 他テナントの client 解決はできない (invalid_client)
 * - exchange code: 別テナントの code 提示は invalid_grant
 * - refresh: 別テナントの refresh token は invalid_grant
 * - device: 別テナントの device_code は invalid_grant
 */

import { describe, expect, it } from 'bun:test'
import { createHash } from 'crypto'

import { authorizeRequestUseCase } from './authorize-request'
import { exchangeCodeForTokenUseCase } from './exchange-code-for-token'
import { exchangeDeviceCodeUseCase } from './exchange-device-code'
import { refreshTokenUseCase } from './refresh-tokens'
import { generateAuthorizationCode } from '../domain/authorization-code'
import { generateInitial as generateInitialRefresh } from '../domain/refresh-token'
import {
  InMemoryAuthorizationCodeStore,
  InMemoryAuthorizationRequestStore,
} from '../../../adapters/persistence/memory/authorization-store'
import { InMemoryClientRepository } from '../../../adapters/persistence/memory/client-repo'
import { InMemoryDeviceCodeStore } from '../../../adapters/persistence/memory/device-code-store'
import { InMemoryRefreshTokenStore } from '../../../adapters/persistence/memory/refresh-store'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { ClientSchema, UserSchema, type Client, type User } from '../../spec-bindings/schemas'
import type { TokenIssuer } from '../ports/token-issuer'
import {
  generateDeviceCode,
  generateUserCode,
  hashDeviceCode,
  normalizeUserCode,
} from '../domain/device-authorization'

class FakeTokenIssuer implements TokenIssuer {
  getAccessTokenTtlSeconds() {
    return 600
  }
  getIdTokenTtlSeconds() {
    return 3600
  }
  async signAccessToken() {
    return { token: 'fake-at', jti: 'jti-0' }
  }
  async signIdToken() {
    return 'fake-id'
  }
}

const VERIFIER = 'verifier-of-known-length-ABCDEFGHIJKLMNOPQRSTUVWXY'
const CHALLENGE = createHash('sha256').update(VERIFIER).digest('base64url')

function makePublicClient(tenant_id: string, client_id: string): Client {
  return ClientSchema.parse({
    tenant_id,
    client_id,
    client_type: 'public',
    redirect_uris: ['https://app.example/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'none',
    scope: 'openid',
    created_at: new Date().toISOString(),
  })
}

function makeUser(tenant_id: string, sub: string): User {
  return UserSchema.parse({
    sub,
    tenant_id,
    preferred_username: sub,
    password_hash: 'pw',
    email: `${sub}@example.com`,
    email_verified: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
}

describe('authorize cannot resolve another tenant client', () => {
  it('default の client_id は acme テナントから見えない (invalid_client)', async () => {
    const clientRepo = new InMemoryClientRepository()
    await clientRepo.save(makePublicClient('default', 'web-app'))
    const requestStore = new InMemoryAuthorizationRequestStore()
    await expect(
      authorizeRequestUseCase(
        { clientRepo, requestStore },
        {
          tenant_id: 'acme',
          client_id: 'web-app',
          redirect_uri: 'https://app.example/cb',
          response_type: 'code',
          scope: 'openid',
          code_challenge: CHALLENGE,
          code_challenge_method: 'S256',
        },
      ),
    ).rejects.toMatchObject({ code: 'unauthorized_client' })
  })
})

describe('authorization_code cannot cross tenant boundary', () => {
  it('acme 発行の code を default で交換すると invalid_grant', async () => {
    const clientRepo = new InMemoryClientRepository()
    const userRepo = new InMemoryUserRepository()
    const codeStore = new InMemoryAuthorizationCodeStore()
    const refreshStore = new InMemoryRefreshTokenStore()
    const tokenIssuer = new FakeTokenIssuer()

    // default 側にだけ client/user を置き、code は acme で発行する
    const client = makePublicClient('default', 'web-app')
    await clientRepo.save(client)
    await userRepo.save(makeUser('default', 'user_alice'))

    const code = generateAuthorizationCode({
      tenant_id: 'acme',
      authorization_request_id: '00000000-0000-0000-0000-000000000001',
      client_id: client.client_id,
      sub: 'user_alice',
      scopes: ['openid'],
      redirect_uri: client.redirect_uris[0],
      code_challenge: CHALLENGE,
      code_challenge_method: 'S256',
      auth_time: 1700000000,
    })
    await codeStore.save(code)

    await expect(
      exchangeCodeForTokenUseCase(
        { clientRepo, userRepo, codeStore, refreshStore, tokenIssuer },
        {
          tenant_id: 'default',
          client_id: client.client_id,
          code: code.code,
          code_verifier: VERIFIER,
          redirect_uri: client.redirect_uris[0],
        },
      ),
    ).rejects.toMatchObject({ code: 'invalid_grant' })
  })
})

describe('refresh_token cannot cross tenant boundary', () => {
  it('acme 発行の refresh を default で使うと invalid_grant', async () => {
    const clientRepo = new InMemoryClientRepository()
    const userRepo = new InMemoryUserRepository()
    const refreshStore = new InMemoryRefreshTokenStore()
    const tokenIssuer = new FakeTokenIssuer()

    const client = makePublicClient('default', 'web-app')
    await clientRepo.save(client)
    await userRepo.save(makeUser('default', 'user_alice'))

    const initial = generateInitialRefresh({
      tenant_id: 'acme',
      client_id: 'web-app',
      sub: 'user_alice',
      scopes: ['openid'],
    })
    await refreshStore.save(initial.record)

    await expect(
      refreshTokenUseCase(
        { clientRepo, userRepo, refreshStore, tokenIssuer },
        {
          tenant_id: 'default',
          client_id: 'web-app',
          refresh_token: initial.token,
        },
      ),
    ).rejects.toMatchObject({ code: 'invalid_grant' })
  })
})

describe('device_code cannot cross tenant boundary', () => {
  it('acme 発行の device_code を default で交換すると invalid_grant', async () => {
    const clientRepo = new InMemoryClientRepository()
    const userRepo = new InMemoryUserRepository()
    const deviceCodeStore = new InMemoryDeviceCodeStore()
    const refreshStore = new InMemoryRefreshTokenStore()
    const tokenIssuer = new FakeTokenIssuer()

    const client = ClientSchema.parse({
      tenant_id: 'default',
      client_id: 'tv-app',
      client_type: 'public',
      redirect_uris: ['https://tv.example/cb'],
      grant_types: ['urn:ietf:params:oauth:grant-type:device_code', 'refresh_token'],
      response_types: [],
      token_endpoint_auth_method: 'none',
      scope: 'openid',
      created_at: new Date().toISOString(),
    })
    await clientRepo.save(client)
    await userRepo.save(makeUser('default', 'user_alice'))

    // acme で device_code を発行する (verify 済み状態にする)
    const deviceCode = generateDeviceCode()
    const userCode = generateUserCode()
    const now = new Date()
    await deviceCodeStore.save({
      tenant_id: 'acme',
      device_code_hash: hashDeviceCode(deviceCode),
      user_code_hash: hashDeviceCode(normalizeUserCode(userCode)),
      client_id: 'tv-app',
      scope: 'openid',
      interval_seconds: 5,
      issued_at: now.toISOString(),
      expires_at: new Date(now.getTime() + 600_000).toISOString(),
      state: 'approved',
      sub: 'user_alice',
      auth_time: Math.floor(now.getTime() / 1000),
    } as unknown as Parameters<typeof deviceCodeStore.save>[0])

    await expect(
      exchangeDeviceCodeUseCase(
        { clientRepo, userRepo, deviceCodeStore, refreshStore, tokenIssuer },
        { tenant_id: 'default', client_id: 'tv-app', device_code: deviceCode },
      ),
    ).rejects.toMatchObject({ code: 'invalid_grant' })
  })
})
