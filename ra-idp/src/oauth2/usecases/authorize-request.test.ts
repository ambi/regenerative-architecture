/**
 * Layer 3 — Application Logic（/authorize ユースケース統合テスト）
 *
 * SCL scenarios の authorize 周辺境界を、HTTP なしで検証する。
 */

import { describe, expect, it } from 'bun:test'
import { createHash } from 'crypto'

import { authorizeRequestUseCase, completeAuthenticationUseCase } from './authorize-request'
import { InMemoryAuthorizationRequestStore } from '../../../adapters/persistence/memory/authorization-store'
import { InMemoryClientRepository } from '../../../adapters/persistence/memory/client-repo'
import { InMemoryConsentRepository } from '../../../adapters/persistence/memory/consent-repo'
import { ClientSchema, type Client } from '../../spec-bindings/schemas'

function makeClient(overrides: Partial<Client> = {}): Client {
  return ClientSchema.parse({
    client_id: 'web-app',
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile offline_access',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
    ...overrides,
  })
}

async function setup(clientOverrides: Partial<Client> = {}) {
  const clientRepo = new InMemoryClientRepository()
  const consentRepo = new InMemoryConsentRepository()
  const requestStore = new InMemoryAuthorizationRequestStore()
  const client = makeClient(clientOverrides)
  await clientRepo.save(client)
  return { clientRepo, consentRepo, requestStore, client }
}

const AUTH_INPUT = {
  client_id: 'web-app',
  redirect_uri: 'https://app.example.com/cb',
  response_type: 'code' as const,
  scope: 'openid profile',
  code_challenge: 'challenge',
  code_challenge_method: 'S256' as const,
}

describe('authorizeRequestUseCase — consent handling', () => {
  it('既存の同意があれば consented まで進み、同意 UI をスキップできる', async () => {
    const { clientRepo, consentRepo, requestStore, client } = await setup()
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })

    const { request } = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, par_used: false },
    )
    const result = await completeAuthenticationUseCase(
      { consentRepo, requestStore },
      request,
      'user_alice',
    )

    expect(result.needsConsent).toBe(false)
    expect(result.request.state).toBe('consented')
  })

  it('prompt=consent は既存同意があっても再同意を要求する', async () => {
    const { clientRepo, consentRepo, requestStore, client } = await setup()
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })

    const { request } = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, prompt: 'consent', par_used: false },
    )
    const result = await completeAuthenticationUseCase(
      { consentRepo, requestStore },
      request,
      'user_alice',
    )

    expect(result.needsConsent).toBe(true)
    expect(result.request.state).toBe('consent_pending')
  })
})

describe('authorizeRequestUseCase — OIDC session prompts', () => {
  it('max_age を超えた認証時刻では再認証を要求する', async () => {
    const { clientRepo, consentRepo, requestStore, client } = await setup()
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })

    const { request } = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, max_age: 60, par_used: false },
    )
    const result = await completeAuthenticationUseCase(
      { consentRepo, requestStore },
      request,
      'user_alice',
      new Date('2026-01-01T00:00:00Z'),
      new Date('2026-01-01T02:00:00Z'),
    )

    expect(result.needsAuthentication).toBe(true)
    expect(result.needsConsent).toBe(false)
    expect(result.request.state).toBe('authentication_pending')
  })

  it('max_age 内の認証時刻では既存同意により consented まで進む', async () => {
    const { clientRepo, consentRepo, requestStore, client } = await setup()
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })

    const { request } = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, max_age: 3600, par_used: false },
    )
    const result = await completeAuthenticationUseCase(
      { consentRepo, requestStore },
      request,
      'user_alice',
      new Date('2026-01-01T01:30:00Z'),
      new Date('2026-01-01T02:00:00Z'),
    )

    expect(result.needsAuthentication).toBe(false)
    expect(result.needsConsent).toBe(false)
    expect(result.request.state).toBe('consented')
  })

  it('prompt=login は既存セッションがあっても再認証を要求する', async () => {
    const { clientRepo, consentRepo, requestStore, client } = await setup()
    await consentRepo.save({
      sub: 'user_alice',
      client_id: client.client_id,
      scopes: ['openid', 'profile'],
      granted_at: new Date().toISOString(),
      expires_at: new Date(Date.now() + 86400_000).toISOString(),
    })

    const { request } = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, prompt: 'login', par_used: false },
    )
    const result = await completeAuthenticationUseCase(
      { consentRepo, requestStore },
      request,
      'user_alice',
    )

    expect(result.needsAuthentication).toBe(true)
    expect(result.needsConsent).toBe(false)
    expect(result.request.state).toBe('authentication_pending')
  })
})

describe('authorizeRequestUseCase — PAR policy', () => {
  it('PAR 必須の FAPI クライアントは PAR なしの直接認可リクエストを拒否される', async () => {
    const { clientRepo, consentRepo, requestStore } = await setup({
      require_pushed_authorization_requests: true,
      fapi_profile: 'fapi_2_security_profile',
    })

    await expect(
      authorizeRequestUseCase(
        { clientRepo, consentRepo, requestStore },
        { ...AUTH_INPUT, par_used: false },
      ),
    ).rejects.toThrow(/invalid_request|par_required_if_fapi/)
  })

  it('PAR 必須の FAPI クライアントは PAR 経由なら認可開始できる', async () => {
    const { clientRepo, consentRepo, requestStore } = await setup({
      require_pushed_authorization_requests: true,
      fapi_profile: 'fapi_2_security_profile',
    })

    const result = await authorizeRequestUseCase(
      { clientRepo, consentRepo, requestStore },
      { ...AUTH_INPUT, par_used: true },
    )

    expect(result.request.state).toBe('authentication_pending')
  })
})
