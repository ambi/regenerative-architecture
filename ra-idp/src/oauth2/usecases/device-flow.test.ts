/**
 * Device Authorization Grant (RFC 8628) のエンドツーエンドテスト。
 *
 * request → (poll: pending) → approve → (poll: tokens) → (poll: invalid_grant)
 * および deny / expired / slow_down / scope / grant 宣言の各分岐を検証する。
 *
 * 永続化は in-memory、署名は実 JoseTokenSigner + InMemoryKeyStore を使う
 * (発行されるトークンが本物の JWT であることまで確認する)。
 */

import { describe, test, expect } from 'bun:test'
import { decodeJwt } from 'jose'

import { ClientSchema, UserSchema, type DomainEvent } from '../../spec-bindings/schemas'
import { InMemoryClientRepository } from '../../../adapters/persistence/memory/client-repo'
import { InMemoryUserRepository } from '../../../adapters/persistence/memory/user-repo'
import { InMemoryDeviceCodeStore } from '../../../adapters/persistence/memory/device-code-store'
import { InMemoryRefreshTokenStore } from '../../../adapters/persistence/memory/refresh-store'
import { InMemoryKeyStore } from '../../../adapters/crypto/in-memory-key-store'
import { JoseTokenSigner } from '../../../adapters/crypto/jwt-signer'

import { requestDeviceAuthorizationUseCase } from './request-device-authorization'
import { verifyUserCodeUseCase } from './verify-user-code'
import { exchangeDeviceCodeUseCase } from './exchange-device-code'
import {
  generateUserCode,
  generateDeviceCode,
  normalizeUserCode,
  hashDeviceCode,
} from '../domain/device-authorization'

const ISSUER = 'https://idp.example.com'
const DEVICE_GRANT = 'urn:ietf:params:oauth:grant-type:device_code'

async function setup() {
  const clientRepo = new InMemoryClientRepository()
  const userRepo = new InMemoryUserRepository()
  const deviceCodeStore = new InMemoryDeviceCodeStore()
  const refreshStore = new InMemoryRefreshTokenStore()
  const keyStore = await InMemoryKeyStore.create('PS256')
  const tokenIssuer = new JoseTokenSigner(ISSUER, keyStore)

  const client = ClientSchema.parse({
    client_id: 'tv-app',
    client_type: 'public',
    client_name: 'Smart TV App',
    redirect_uris: ['https://tv.example.com/cb'],
    grant_types: [DEVICE_GRANT, 'refresh_token'],
    response_types: [],
    token_endpoint_auth_method: 'none',
    scope: 'openid profile',
    created_at: new Date().toISOString(),
  })
  await clientRepo.save(client)

  const user = UserSchema.parse({
    sub: 'user_alice',
    preferred_username: 'alice',
    password_hash: 'pw',
    email: 'alice@example.com',
    email_verified: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
  await userRepo.save(user)

  const events: DomainEvent[] = []
  const emit = (e: DomainEvent) => events.push(e)

  return { clientRepo, userRepo, deviceCodeStore, refreshStore, tokenIssuer, client, events, emit }
}

describe('domain/device-authorization', () => {
  test('user_code はハイフン区切り・許可文字のみ', () => {
    for (let i = 0; i < 50; i++) {
      const uc = generateUserCode()
      expect(uc).toMatch(/^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$/)
      expect(normalizeUserCode(uc)).toMatch(/^[BCDFGHJKLMNPQRSTVWXZ]{8}$/)
    }
  })

  test('device_code は一意・高エントロピーでハッシュ保存できる', () => {
    const a = generateDeviceCode()
    const b = generateDeviceCode()
    expect(a).not.toBe(b)
    expect(a.length).toBeGreaterThanOrEqual(40)
    expect(hashDeviceCode(a)).toHaveLength(64) // sha256 hex
  })
})

describe('Device Authorization Grant — happy path', () => {
  test('request → pending → approve → tokens → replay は invalid_grant', async () => {
    const d = await setup()
    const t0 = new Date('2026-01-01T00:00:00Z')

    // 1) request
    const { response } = await requestDeviceAuthorizationUseCase(
      { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
      { client: d.client, scope: 'openid' },
      t0,
    )
    expect(response.device_code.length).toBeGreaterThan(10)
    expect(response.verification_uri).toBe(`${ISSUER}/device`)
    expect(response.interval).toBe(5)

    // 2) poll before approval → authorization_pending
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        t0,
      ),
    ).rejects.toMatchObject({ code: 'authorization_pending' })

    // 3) approve at verification_uri
    const approve = await verifyUserCodeUseCase(
      { deviceCodeStore: d.deviceCodeStore },
      { user_code: response.user_code, sub: 'user_alice', auth_time: 1735689600, action: 'allow' },
      d.emit,
      t0,
    )
    expect(approve.result).toBe('approved')

    // 4) poll after approval (interval 経過後) → tokens
    const t1 = new Date(t0.getTime() + 10_000)
    const { response: tokens, audit } = await exchangeDeviceCodeUseCase(
      d,
      { client_id: d.client.client_id, device_code: response.device_code },
      t1,
    )
    expect(tokens.access_token).toBeTruthy()
    expect(tokens.refresh_token).toBeTruthy()
    expect(tokens.id_token).toBeTruthy() // openid scope
    expect(tokens.token_type).toBe('Bearer')
    expect(audit.sub).toBe('user_alice')

    // 発行された access_token は本物の JWT で sub を含む
    const claims = decodeJwt(tokens.access_token)
    expect(claims.sub).toBe('user_alice')

    // 5) replay (再交換) → invalid_grant
    const t2 = new Date(t1.getTime() + 10_000)
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        t2,
      ),
    ).rejects.toMatchObject({ code: 'invalid_grant' })

    // 監査イベント: Approved が発行されている
    expect(d.events.some((e) => e.type === 'DeviceAuthorizationApproved')).toBe(true)
  })
})

describe('Device Authorization Grant — エラー分岐', () => {
  test('ユーザー拒否 → access_denied', async () => {
    const d = await setup()
    const t0 = new Date('2026-01-01T00:00:00Z')
    const { response } = await requestDeviceAuthorizationUseCase(
      { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
      { client: d.client, scope: 'openid' },
      t0,
    )
    await verifyUserCodeUseCase(
      { deviceCodeStore: d.deviceCodeStore },
      { user_code: response.user_code, sub: 'user_alice', auth_time: 1, action: 'deny' },
      d.emit,
      t0,
    )
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        t0,
      ),
    ).rejects.toMatchObject({ code: 'access_denied' })
    expect(d.events.some((e) => e.type === 'DeviceAuthorizationDenied')).toBe(true)
  })

  test('期限切れ device_code → expired_token', async () => {
    const d = await setup()
    const t0 = new Date('2026-01-01T00:00:00Z')
    const { response } = await requestDeviceAuthorizationUseCase(
      { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
      { client: d.client, scope: 'openid' },
      t0,
    )
    const tLate = new Date(t0.getTime() + 700_000) // > 600s TTL
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        tLate,
      ),
    ).rejects.toMatchObject({ code: 'expired_token' })
  })

  test('interval より速いポーリング → slow_down', async () => {
    const d = await setup()
    const t0 = new Date('2026-01-01T00:00:00Z')
    const { response } = await requestDeviceAuthorizationUseCase(
      { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
      { client: d.client, scope: 'openid' },
      t0,
    )
    // 1 回目: pending (last_polled_at を記録)
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        t0,
      ),
    ).rejects.toMatchObject({ code: 'authorization_pending' })
    // 2 回目: 1 秒後 (interval 5s 未満) → slow_down
    const t1 = new Date(t0.getTime() + 1000)
    await expect(
      exchangeDeviceCodeUseCase(
        d,
        { client_id: d.client.client_id, device_code: response.device_code },
        t1,
      ),
    ).rejects.toMatchObject({ code: 'slow_down' })
  })

  test('未知の device_code → invalid_grant', async () => {
    const d = await setup()
    await expect(
      exchangeDeviceCodeUseCase(d, { client_id: d.client.client_id, device_code: 'nope' }),
    ).rejects.toMatchObject({ code: 'invalid_grant' })
  })

  test('device_code グラント未宣言のクライアント → unauthorized_client', async () => {
    const d = await setup()
    const noDevice = ClientSchema.parse({
      client_id: 'web',
      client_type: 'confidential',
      redirect_uris: ['https://w/cb'],
      grant_types: ['authorization_code'],
      response_types: ['code'],
      token_endpoint_auth_method: 'client_secret_basic',
      scope: 'openid',
      created_at: new Date().toISOString(),
    })
    await expect(
      requestDeviceAuthorizationUseCase(
        { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
        { client: noDevice },
      ),
    ).rejects.toMatchObject({ code: 'unauthorized_client' })
  })

  test('宣言外スコープ → invalid_scope', async () => {
    const d = await setup()
    await expect(
      requestDeviceAuthorizationUseCase(
        { deviceCodeStore: d.deviceCodeStore, issuer: ISSUER },
        { client: d.client, scope: 'openid admin:all' },
      ),
    ).rejects.toMatchObject({ code: 'invalid_scope' })
  })
})
