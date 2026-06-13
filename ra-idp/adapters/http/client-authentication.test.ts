/**
 * private_key_jwt (RFC 7523) クライアント認証の単体テスト。
 *
 * verifyClientAssertion は HTTP Context に依存しない純粋関数として切り出してあるため、
 * ここでは jose で実際のクライアント鍵を生成し、SignJWT で client_assertion を作って
 * 検証経路を端から端まで確認する。
 */

import { describe, test, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { generateKeyPair, exportJWK, SignJWT } from 'jose'
import type { JWK } from 'jose'
import { authenticateClient, verifyClientAssertion } from './client-authentication'
import { InMemoryClientAssertionReplayStore } from '../persistence/memory/client-assertion-replay-store'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { ClientSchema, type Client } from '../../src/spec-bindings/schemas'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'
import {
  TEST_CERT_PEM,
  TEST_CERT_OTHER_PEM,
  TEST_CERT_SUBJECT_DN,
} from '../crypto/mtls-test-fixtures'

const ISSUER = 'https://idp.example.com'
const CLIENT_ID = 'svc-private-key-jwt'
const AUDIENCES = [ISSUER, `${ISSUER}/token`]

async function setup() {
  const { publicKey, privateKey } = await generateKeyPair('ES256', { extractable: true })
  const publicJwk = (await exportJWK(publicKey)) as JWK
  publicJwk.kid = 'client-key-1'
  publicJwk.use = 'sig'
  publicJwk.alg = 'ES256'

  const client: Client = ClientSchema.parse({
    tenant_id: 'default',
    client_id: CLIENT_ID,
    client_type: 'confidential',
    client_name: 'Private Key JWT Service',
    redirect_uris: ['https://svc.example.com/cb'],
    grant_types: ['client_credentials'],
    response_types: [],
    token_endpoint_auth_method: 'private_key_jwt',
    scope: 'api',
    jwks: { keys: [publicJwk] },
    created_at: new Date().toISOString(),
  })

  const repo: ClientRepository = {
    async findById(_tenant_id, id) {
      return id === CLIENT_ID ? client : null
    },
    async save() {},
    async delete() {},
    async findAll() {
      return [client]
    },
  }

  async function makeAssertion(over: Record<string, unknown> = {}, ttlSeconds = 120) {
    const nowSec = Math.floor(Date.now() / 1000)
    const payload: Record<string, unknown> = {
      jti: `jti-${Math.random().toString(36).slice(2)}`,
      ...over,
    }
    return await new SignJWT(payload)
      .setProtectedHeader({ alg: 'ES256', kid: 'client-key-1' })
      .setIssuer(CLIENT_ID)
      .setSubject(CLIENT_ID)
      .setAudience(`${ISSUER}/token`)
      .setIssuedAt(nowSec)
      .setExpirationTime(nowSec + ttlSeconds)
      .sign(privateKey)
  }

  return { client, repo, makeAssertion, privateKey }
}

describe('verifyClientAssertion (private_key_jwt)', () => {
  test('正当な assertion を受理しクライアントを返す', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion()
    const client = await verifyClientAssertion(assertion, repo, {
      tenantId: 'default',
      audiences: AUDIENCES,
      replayStore: store,
    })
    expect(client.client_id).toBe(CLIENT_ID)
  })

  test('同じ jti の再使用はリプレイとして拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({ jti: 'fixed-jti' })
    await verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store })
    // 同一トークンの 2 度目
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('aud が一致しない assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion()
    await expect(
      verifyClientAssertion(assertion, repo, {
        tenantId: 'default',
        audiences: ['https://other.example.com'],
        replayStore: store,
      }),
    ).rejects.toThrow(OAuthError)
  })

  test('期限切れ assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({}, -3600) // exp = now - 1h
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('寿命が長すぎる assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({}, 24 * 3600) // 24h
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('iss/sub が client_id と異なる assertion を拒否する', async () => {
    const { repo } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const { privateKey } = await generateKeyPair('ES256', { extractable: true })
    const nowSec = Math.floor(Date.now() / 1000)
    const assertion = await new SignJWT({ jti: 'x' })
      .setProtectedHeader({ alg: 'ES256' })
      .setIssuer('someone-else')
      .setSubject('someone-else')
      .setAudience(`${ISSUER}/token`)
      .setIssuedAt(nowSec)
      .setExpirationTime(nowSec + 120)
      .sign(privateKey)
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('別の鍵で署名された assertion を拒否する', async () => {
    const { repo } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const { privateKey: attackerKey } = await generateKeyPair('ES256', { extractable: true })
    const nowSec = Math.floor(Date.now() / 1000)
    const assertion = await new SignJWT({ jti: 'y' })
      .setProtectedHeader({ alg: 'ES256' })
      .setIssuer(CLIENT_ID)
      .setSubject(CLIENT_ID)
      .setAudience(`${ISSUER}/token`)
      .setIssuedAt(nowSec)
      .setExpirationTime(nowSec + 120)
      .sign(attackerKey)
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('jti を持たない assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({ jti: undefined })
    await expect(
      verifyClientAssertion(assertion, repo, { tenantId: 'default',
        audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })
})

/**
 * tls_client_auth (RFC 8705 §2.1.2) のクライアント認証。
 *
 * authenticateClient は HTTP Context に依存するため、Hono の最小ルートを組んで
 * X-Client-Certificate ヘッダ付きのリクエストを通す。
 */
describe('authenticateClient + tls_client_auth (RFC 8705 §2.1.2)', () => {
  const MTLS_CLIENT_ID = 'mtls-app'

  function makeMtlsClient(overrides: Partial<Client> = {}): Client {
    return ClientSchema.parse({
      tenant_id: 'default',
      client_id: MTLS_CLIENT_ID,
      client_type: 'confidential',
      redirect_uris: ['https://app.example.com/cb'],
      grant_types: ['client_credentials'],
      response_types: [],
      token_endpoint_auth_method: 'tls_client_auth',
      scope: 'api',
      tls_client_auth_subject_dn: TEST_CERT_SUBJECT_DN,
      created_at: new Date().toISOString(),
      ...overrides,
    })
  }

  function makeRepo(client: Client): ClientRepository {
    return {
      async findById(_tenant_id, id) {
        return id === client.client_id ? client : null
      },
      async save() {},
      async delete() {},
      async findAll() {
        return [client]
      },
    }
  }

  async function postWith(
    headers: Record<string, string>,
    body: string,
    repo: ClientRepository,
  ): Promise<{ ok: boolean; error?: string; thumbprint?: string }> {
    const app = new Hono()
    app.post('/auth', async (c) => {
      try {
        const parsed = Object.fromEntries(new URLSearchParams(await c.req.text()).entries())
        const auth = await authenticateClient(c, parsed, repo)
        return c.json({ ok: true, thumbprint: auth.mtlsThumbprintS256 })
      } catch (e) {
        if (e instanceof OAuthError) return c.json({ ok: false, error: e.message }, 400)
        throw e
      }
    })
    const res = await app.request('/auth', {
      method: 'POST',
      headers: { 'content-type': 'application/x-www-form-urlencoded', ...headers },
      body,
    })
    return (await res.json()) as { ok: boolean; error?: string; thumbprint?: string }
  }

  it('登録 DN と一致する証明書で認証成功し thumbprint を返す', async () => {
    const repo = makeRepo(makeMtlsClient())
    const r = await postWith(
      { 'X-Client-Certificate': encodeURIComponent(TEST_CERT_PEM) },
      `client_id=${MTLS_CLIENT_ID}`,
      repo,
    )
    expect(r.ok).toBe(true)
    expect(r.thumbprint).toBeTruthy()
  })

  it('別 DN の証明書は invalid_client', async () => {
    const repo = makeRepo(makeMtlsClient())
    const r = await postWith(
      { 'X-Client-Certificate': encodeURIComponent(TEST_CERT_OTHER_PEM) },
      `client_id=${MTLS_CLIENT_ID}`,
      repo,
    )
    expect(r.ok).toBe(false)
    expect(r.error).toContain('subject DN')
  })

  it('登録 DN 未設定のクライアントは invalid_client', async () => {
    const repo = makeRepo(makeMtlsClient({ tls_client_auth_subject_dn: undefined }))
    const r = await postWith(
      { 'X-Client-Certificate': encodeURIComponent(TEST_CERT_PEM) },
      `client_id=${MTLS_CLIENT_ID}`,
      repo,
    )
    expect(r.ok).toBe(false)
    expect(r.error).toContain('tls_client_auth_subject_dn')
  })

  it('証明書ヘッダ無しの tls_client_auth クライアントは認証不可', async () => {
    const repo = makeRepo(makeMtlsClient())
    const r = await postWith({}, `client_id=${MTLS_CLIENT_ID}`, repo)
    expect(r.ok).toBe(false)
  })

  it('壊れた証明書は invalid_client', async () => {
    const repo = makeRepo(makeMtlsClient())
    const r = await postWith(
      { 'X-Client-Certificate': encodeURIComponent('not a cert') },
      `client_id=${MTLS_CLIENT_ID}`,
      repo,
    )
    expect(r.ok).toBe(false)
  })
})
