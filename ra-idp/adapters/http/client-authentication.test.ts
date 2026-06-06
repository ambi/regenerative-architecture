/**
 * private_key_jwt (RFC 7523) クライアント認証の単体テスト。
 *
 * verifyClientAssertion は HTTP Context に依存しない純粋関数として切り出してあるため、
 * ここでは jose で実際のクライアント鍵を生成し、SignJWT で client_assertion を作って
 * 検証経路を端から端まで確認する。
 */

import { describe, test, expect } from 'bun:test'
import { generateKeyPair, exportJWK, SignJWT } from 'jose'
import type { JWK } from 'jose'
import { verifyClientAssertion } from './client-authentication'
import { InMemoryClientAssertionReplayStore } from '../persistence/memory/client-assertion-replay-store'
import { OAuthError } from '../../src/oauth2/protocol/oauth-error'
import { ClientSchema, type Client } from '../../src/spec-bindings/schemas'
import type { ClientRepository } from '../../src/oauth2/ports/client-repository'

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
    async findById(id) {
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
      audiences: AUDIENCES,
      replayStore: store,
    })
    expect(client.client_id).toBe(CLIENT_ID)
  })

  test('同じ jti の再使用はリプレイとして拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({ jti: 'fixed-jti' })
    await verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store })
    // 同一トークンの 2 度目
    await expect(
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('aud が一致しない assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion()
    await expect(
      verifyClientAssertion(assertion, repo, {
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
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('寿命が長すぎる assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({}, 24 * 3600) // 24h
    await expect(
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
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
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
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
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })

  test('jti を持たない assertion を拒否する', async () => {
    const { repo, makeAssertion } = await setup()
    const store = new InMemoryClientAssertionReplayStore()
    const assertion = await makeAssertion({ jti: undefined })
    await expect(
      verifyClientAssertion(assertion, repo, { audiences: AUDIENCES, replayStore: store }),
    ).rejects.toThrow(OAuthError)
  })
})
