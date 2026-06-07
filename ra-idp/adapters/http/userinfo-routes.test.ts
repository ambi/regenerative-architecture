/**
 * Layer 4 — Adapter Layer（HTTP: /userinfo）
 *
 * RFC 9449 §7.1: DPoP-bound access token を /userinfo で利用する場合、
 *   - Authorization scheme は DPoP
 *   - 各リクエストで DPoP proof が必須
 *   - proof.payload.ath が SHA-256(access_token) と一致
 *   - proof.jwk のサムプリントが AT.cnf.jkt と一致
 */

import { describe, expect, it } from 'bun:test'
import { Hono } from 'hono'
import { createHash, randomUUID } from 'crypto'
import { SignJWT, exportJWK, generateKeyPair, calculateJwkThumbprint } from 'jose'
import type { JWK, KeyLike } from 'jose'

import { createUserInfoRoutes } from './userinfo-routes'
import { InMemoryDpopReplayStore } from '../persistence/memory/dpop-replay-store'
import { InMemoryUserRepository } from '../persistence/memory/user-repo'
import { InMemoryKeyStore } from '../crypto/in-memory-key-store'
import { HmacDpopNonceService } from '../crypto/hmac-dpop-nonce-service'
import { JoseTokenSigner } from '../crypto/jwt-signer'
import { ClientSchema, UserSchema, type Client } from '../../src/spec-bindings/schemas'

const ISSUER = 'http://idp.example.com'

function makeClient(): Client {
  return ClientSchema.parse({
    client_id: 'web-app',
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: true,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
  })
}

async function setup() {
  const userRepo = new InMemoryUserRepository()
  const keyStore = await InMemoryKeyStore.create('PS256')
  const signer = new JoseTokenSigner(ISSUER, keyStore)
  const dpopReplayStore = new InMemoryDpopReplayStore()
  const dpopNonceService = HmacDpopNonceService.withRandomSecret(60)
  const client = makeClient()
  await userRepo.save(
    UserSchema.parse({
      sub: 'user_alice',
      preferred_username: 'alice',
      password_hash: 'x',
      email: 'alice@example.com',
      email_verified: true,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }),
  )

  const app = new Hono()
  app.route(
    '/',
    createUserInfoRoutes({
      issuer: ISSUER,
      introspector: signer,
      userRepo,
      dpopReplayStore,
      dpopNonceService,
    }),
  )

  return { app, signer, client, dpopNonceService }
}

async function makeDpopKey() {
  const { publicKey, privateKey } = await generateKeyPair('ES256', { extractable: true })
  const jwk = await exportJWK(publicKey)
  jwk.alg = 'ES256'
  const jkt = await calculateJwkThumbprint(jwk)
  return { privateKey, jwk, jkt }
}

async function makeDpopProof(opts: {
  privateKey: KeyLike
  jwk: JWK
  htm: string
  htu: string
  ath?: string
  nonce?: string
  jti?: string
}): Promise<string> {
  const payload: Record<string, unknown> = { htm: opts.htm, htu: opts.htu }
  if (opts.ath) payload.ath = opts.ath
  if (opts.nonce) payload.nonce = opts.nonce
  return new SignJWT(payload)
    .setProtectedHeader({ typ: 'dpop+jwt', alg: 'ES256', jwk: opts.jwk })
    .setIssuedAt()
    .setJti(opts.jti ?? randomUUID())
    .sign(opts.privateKey)
}

describe('/userinfo + DPoP-bound access token', () => {
  it('成功: 有効な DPoP proof + nonce で 200 を返す', async () => {
    const { app, signer, client, dpopNonceService } = await setup()
    const dpop = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid', 'profile'],
      senderConstraint: { type: 'dpop', jkt: dpop.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const ath = createHash('sha256').update(token).digest('base64url')
    const proof = await makeDpopProof({
      privateKey: dpop.privateKey,
      jwk: dpop.jwk,
      htm: 'GET',
      htu: `${ISSUER}/userinfo`,
      ath,
      nonce: dpopNonceService.issue(),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `DPoP ${token}`, DPoP: proof },
    })
    expect(res.status).toBe(200)
    expect(res.headers.get('DPoP-Nonce')).toBeTruthy()
    const body = (await res.json()) as Record<string, unknown>
    expect(body.sub).toBe('user_alice')
  })

  it('Bearer scheme で DPoP-bound AT を送ると invalid_token', async () => {
    const { app, signer, client } = await setup()
    const dpop = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'dpop', jkt: dpop.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status).toBe(400)
    expect(res.headers.get('WWW-Authenticate')).toContain('DPoP')
  })

  it('ath 不一致は invalid_dpop_proof で拒否される', async () => {
    const { app, signer, client, dpopNonceService } = await setup()
    const dpop = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'dpop', jkt: dpop.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const wrongAth = createHash('sha256').update('別のトークン').digest('base64url')
    const proof = await makeDpopProof({
      privateKey: dpop.privateKey,
      jwk: dpop.jwk,
      htm: 'GET',
      htu: `${ISSUER}/userinfo`,
      ath: wrongAth,
      nonce: dpopNonceService.issue(),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `DPoP ${token}`, DPoP: proof },
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string }
    expect(body.error).toBe('invalid_dpop_proof')
  })

  it('別の DPoP 鍵で署名された proof は jkt 不一致で拒否される', async () => {
    const { app, signer, client, dpopNonceService } = await setup()
    const bound = await makeDpopKey()
    const attacker = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'dpop', jkt: bound.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const ath = createHash('sha256').update(token).digest('base64url')
    const proof = await makeDpopProof({
      privateKey: attacker.privateKey,
      jwk: attacker.jwk,
      htm: 'GET',
      htu: `${ISSUER}/userinfo`,
      ath,
      nonce: dpopNonceService.issue(),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `DPoP ${token}`, DPoP: proof },
    })
    expect(res.status).toBe(400)
  })

  it('nonce 無しの DPoP proof は use_dpop_nonce + DPoP-Nonce ヘッダー付きで拒否される', async () => {
    const { app, signer, client } = await setup()
    const dpop = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'dpop', jkt: dpop.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const ath = createHash('sha256').update(token).digest('base64url')
    const proof = await makeDpopProof({
      privateKey: dpop.privateKey,
      jwk: dpop.jwk,
      htm: 'GET',
      htu: `${ISSUER}/userinfo`,
      ath,
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `DPoP ${token}`, DPoP: proof },
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string }
    expect(body.error).toBe('use_dpop_nonce')
    expect(res.headers.get('DPoP-Nonce')).toBeTruthy()
  })

  it('改ざんされた nonce は use_dpop_nonce で拒否される', async () => {
    const { app, signer, client } = await setup()
    const dpop = await makeDpopKey()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'dpop', jkt: dpop.jkt },
      authTime: Math.floor(Date.now() / 1000),
    })
    const ath = createHash('sha256').update(token).digest('base64url')
    const proof = await makeDpopProof({
      privateKey: dpop.privateKey,
      jwk: dpop.jwk,
      htm: 'GET',
      htu: `${ISSUER}/userinfo`,
      ath,
      nonce: 'forged-nonce',
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `DPoP ${token}`, DPoP: proof },
    })
    expect(res.status).toBe(400)
    const body = (await res.json()) as { error: string }
    expect(body.error).toBe('use_dpop_nonce')
  })

  it('cnf 無し (非バインド) AT は Bearer scheme で従来通り通る', async () => {
    const { app, signer, client } = await setup()
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: null,
      authTime: Math.floor(Date.now() / 1000),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status).toBe(200)
  })
})

describe('/userinfo + mTLS-bound access token (RFC 8705 §3)', () => {
  it('成功: cnf.x5t#S256 と一致する X-Client-Certificate で 200', async () => {
    const { app, signer, client } = await setup()
    const { TEST_CERT_PEM } = await import('../crypto/mtls-test-fixtures')
    const { X509Certificate, createHash } = await import('crypto')
    const der = new X509Certificate(TEST_CERT_PEM).raw
    const thumbprint = createHash('sha256').update(der).digest('base64url')
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid', 'profile'],
      senderConstraint: { type: 'mtls', 'x5t#S256': thumbprint },
      authTime: Math.floor(Date.now() / 1000),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Client-Certificate': encodeURIComponent(TEST_CERT_PEM),
      },
    })
    expect(res.status).toBe(200)
    const body = (await res.json()) as Record<string, unknown>
    expect(body.sub).toBe('user_alice')
  })

  it('証明書ヘッダ無しは invalid_token で拒否', async () => {
    const { app, signer, client } = await setup()
    const { TEST_CERT_PEM } = await import('../crypto/mtls-test-fixtures')
    const { X509Certificate, createHash } = await import('crypto')
    const thumbprint = createHash('sha256')
      .update(new X509Certificate(TEST_CERT_PEM).raw)
      .digest('base64url')
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'mtls', 'x5t#S256': thumbprint },
      authTime: Math.floor(Date.now() / 1000),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status).toBe(400)
    expect(res.headers.get('WWW-Authenticate')).toContain('invalid_token')
  })

  it('別証明書で提示すると thumbprint 不一致で拒否', async () => {
    const { app, signer, client } = await setup()
    const { TEST_CERT_PEM, TEST_CERT_OTHER_PEM } = await import(
      '../crypto/mtls-test-fixtures'
    )
    const { X509Certificate, createHash } = await import('crypto')
    const boundThumb = createHash('sha256')
      .update(new X509Certificate(TEST_CERT_PEM).raw)
      .digest('base64url')
    const { token } = await signer.signAccessToken({
      sub: 'user_alice',
      client,
      scopes: ['openid'],
      senderConstraint: { type: 'mtls', 'x5t#S256': boundThumb },
      authTime: Math.floor(Date.now() / 1000),
    })
    const res = await app.request('/userinfo', {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
        'X-Client-Certificate': encodeURIComponent(TEST_CERT_OTHER_PEM),
      },
    })
    expect(res.status).toBe(400)
  })
})
