/**
 * Layer 4 — Adapter Layer（HTTP: /authorize / /end_session）
 *
 * OIDC セッション系シナリオを HTTP 入力境界で検証する。
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { createHash } from 'crypto'

import { createAuthorizeRoutes } from './authorize-routes'
import {
  InMemoryAuthorizationCodeStore,
  InMemoryAuthorizationRequestStore,
  InMemoryPARStore,
} from '../persistence/memory/authorization-store'
import { InMemoryClientRepository } from '../persistence/memory/client-repo'
import { InMemoryConsentRepository } from '../persistence/memory/consent-repo'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import {
  ClientSchema,
  UserSchema,
  type Client,
  type DomainEvent,
} from '../../src/spec-bindings/schemas'

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

async function setup() {
  const clientRepo = new InMemoryClientRepository()
  const userRepo = new InMemoryUserRepository()
  const consentRepo = new InMemoryConsentRepository()
  const requestStore = new InMemoryAuthorizationRequestStore()
  const codeStore = new InMemoryAuthorizationCodeStore()
  const parStore = new InMemoryPARStore()
  const events: DomainEvent[] = []
  const client = makeClient()

  await clientRepo.save(client)
  await userRepo.save(
    UserSchema.parse({
      sub: 'user_alice',
      preferred_username: 'alice',
      password_hash: createHash('sha256').update('pw').digest('hex'),
      email: 'alice@example.com',
      email_verified: true,
      mfa_enrolled: false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }),
  )
  await consentRepo.save({
    sub: 'user_alice',
    client_id: client.client_id,
    scopes: ['openid', 'profile'],
    granted_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 86400_000).toISOString(),
  })

  const app = new Hono()
  app.route(
    '/',
    createAuthorizeRoutes({
      clientRepo,
      userRepo,
      consentRepo,
      requestStore,
      codeStore,
      parStore,
      emit: (e) => events.push(e),
    }),
  )

  return { app, events }
}

function authorizeUrl(extra: Record<string, string> = {}): string {
  const url = new URL('http://idp.example.com/authorize')
  url.searchParams.set('client_id', 'web-app')
  url.searchParams.set('redirect_uri', 'https://app.example.com/cb')
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'openid profile')
  url.searchParams.set('code_challenge', 'challenge')
  url.searchParams.set('code_challenge_method', 'S256')
  for (const [k, v] of Object.entries(extra)) url.searchParams.set(k, v)
  return url.toString()
}

describe('authorize routes — OIDC session prompts', () => {
  it('prompt=none は未認証時に access_denied を返す', async () => {
    const { app } = await setup()

    const res = await app.request(authorizeUrl({ prompt: 'none' }))

    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'access_denied' })
  })

  it('X-User-Auth-Time が max_age を超えている場合はログインを要求する', async () => {
    const { app } = await setup()

    const res = await app.request(authorizeUrl({ max_age: '60' }), {
      headers: {
        'X-User-Sub': 'user_alice',
        'X-User-Auth-Time': String(Math.floor(Date.now() / 1000) - 120),
      },
    })

    expect(res.status).toBe(401)
    expect(await res.text()).toContain('ログインが必要です')
  })
})

describe('authorize routes — RP-Initiated Logout', () => {
  it('end_session は登録済み post_logout_redirect_uri に state 付きでリダイレクトする', async () => {
    const { app } = await setup()

    const res = await app.request(
      'http://idp.example.com/end_session?client_id=web-app&post_logout_redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb&state=s1',
    )

    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toBe('https://app.example.com/cb?state=s1')
  })

  it('end_session は未登録 post_logout_redirect_uri を拒否する', async () => {
    const { app } = await setup()

    const res = await app.request(
      'http://idp.example.com/end_session?client_id=web-app&post_logout_redirect_uri=https%3A%2F%2Fevil.example.com%2Fcb',
    )

    expect(res.status).toBe(400)
    expect(await res.json()).toMatchObject({ error: 'invalid_request' })
  })
})
