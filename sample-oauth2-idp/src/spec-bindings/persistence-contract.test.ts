/**
 * Layer 3 — Specification Binding (contract tests)
 *
 * 永続化アダプタの契約テスト。同じテスト群を:
 *   - InMemory*  (常に実行)
 *   - Postgres*  (DATABASE_URL が設定されているときのみ)
 *   - Redis*     (REDIS_URL が設定されているときのみ)
 * の 3 種類に対して回す。
 *
 * このテストが「ADR-003 (Adapter Replaceability) の実コード証明」。
 * 同じ仕様 (ポート I/F) に従う限り、永続化を差し替えても
 * src/usecases/* と adapters/http/* は無変更で動くことを保証する。
 */

import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { createHash, randomUUID, randomBytes } from 'crypto'

import {
  ClientSchema,
  UserSchema,
  ConsentSchema,
  RefreshTokenRecordSchema,
  AuthorizationCodeSchema,
  PARRecordSchema,
} from './schemas'
import type {
  Client,
  User,
  Consent,
  RefreshTokenRecord,
  AuthorizationCode,
  PARRecord,
} from './schemas'

import { InMemoryClientRepository } from '../../adapters/persistence/memory/client-repo'
import { InMemoryUserRepository } from '../../adapters/persistence/memory/user-repo'
import { InMemoryConsentRepository } from '../../adapters/persistence/memory/consent-repo'
import { InMemoryRefreshTokenStore } from '../../adapters/persistence/memory/refresh-store'
import {
  InMemoryAuthorizationCodeStore,
  InMemoryPARStore,
} from '../../adapters/persistence/memory/authorization-store'
import { InMemoryDpopReplayStore } from '../../adapters/persistence/memory/dpop-replay-store'
import { InMemoryKeyStore } from '../../adapters/crypto/in-memory-key-store'

import type { ClientRepository } from '../ports/client-repository'
import type { UserRepository } from '../ports/user-repository'
import type { ConsentRepository } from '../ports/consent-repository'
import type { RefreshTokenStore } from '../ports/refresh-token-store'
import type { AuthorizationCodeStore, PARStore } from '../ports/authorization-store'
import type { DpopReplayStore } from '../ports/dpop-replay-store'
import type { KeyStore } from '../ports/key-store'

// ---------------------------------------------------------------
// 共通フィクスチャ
// ---------------------------------------------------------------
function makeClient(id = 'test-client'): Client {
  return ClientSchema.parse({
    client_id: id,
    client_secret_hash: createHash('sha256').update('s').digest('hex'),
    client_type: 'confidential',
    redirect_uris: ['https://app.example.com/cb'],
    grant_types: ['authorization_code', 'refresh_token'],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
  })
}

function makeUser(sub = 'user_x'): User {
  return UserSchema.parse({
    sub,
    preferred_username: `user-${sub}`,
    password_hash: 'pw-hash',
    email: `${sub}@example.com`,
    email_verified: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
}

function makeConsent(sub: string, client_id: string): Consent {
  const now = new Date()
  return ConsentSchema.parse({
    sub,
    client_id,
    scopes: ['openid', 'profile'],
    granted_at: now.toISOString(),
    expires_at: new Date(now.getTime() + 86400_000).toISOString(),
  })
}

function makeRefresh(client_id: string, sub: string, family_id?: string): RefreshTokenRecord {
  const now = new Date()
  return RefreshTokenRecordSchema.parse({
    id: randomUUID(),
    hash: createHash('sha256').update(randomBytes(48)).digest('hex'),
    family_id: family_id ?? randomUUID(),
    client_id,
    sub,
    scopes: ['openid', 'profile'],
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + 14 * 86400_000).toISOString(),
    absolute_expires_at: new Date(now.getTime() + 30 * 86400_000).toISOString(),
    revoked: false,
    rotated: false,
    sender_constraint: null,
  })
}

function makeAuthCode(client_id: string, sub: string): AuthorizationCode {
  const now = new Date()
  return AuthorizationCodeSchema.parse({
    code: randomBytes(32).toString('base64url'),
    authorization_request_id: randomUUID(),
    client_id,
    sub,
    scopes: ['openid', 'profile'],
    redirect_uri: 'https://app.example.com/cb',
    code_challenge: createHash('sha256').update('verifier').digest('base64url'),
    code_challenge_method: 'S256',
    auth_time: Math.floor(now.getTime() / 1000),
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + 60_000).toISOString(),
  })
}

function makePAR(client_id: string): PARRecord {
  const now = new Date()
  return PARRecordSchema.parse({
    request_uri: `urn:ietf:params:oauth:request_uri:${randomBytes(16).toString('hex')}`,
    client_id,
    parameters: { redirect_uri: 'https://app.example.com/cb', scope: 'openid' },
    issued_at: now.toISOString(),
    expires_at: new Date(now.getTime() + 600_000).toISOString(),
    used: false,
  })
}

// ---------------------------------------------------------------
// 契約テスト本体: 全 store を同じテストで検証する
// ---------------------------------------------------------------
interface SuiteFactory {
  name: string
  setup: () => Promise<{
    clientRepo: ClientRepository
    userRepo: UserRepository
    consentRepo: ConsentRepository
    refreshStore: RefreshTokenStore
    codeStore: AuthorizationCodeStore
    parStore: PARStore
    dpopReplayStore: DpopReplayStore
    keyStore: KeyStore
  }>
  teardown?: () => Promise<void>
  skip?: () => boolean
}

const SUITES: SuiteFactory[] = []

// InMemory (常に実行)
SUITES.push({
  name: 'in-memory',
  setup: async () => ({
    clientRepo: new InMemoryClientRepository(),
    userRepo: new InMemoryUserRepository(),
    consentRepo: new InMemoryConsentRepository(),
    refreshStore: new InMemoryRefreshTokenStore(),
    codeStore: new InMemoryAuthorizationCodeStore(),
    parStore: new InMemoryPARStore(),
    dpopReplayStore: new InMemoryDpopReplayStore(),
    keyStore: await InMemoryKeyStore.create('PS256'),
  }),
})

// Postgres + Redis (環境変数が揃っているときのみ)
const dbUrl = process.env.DATABASE_URL
const redisUrl = process.env.REDIS_URL
if (dbUrl && redisUrl) {
  SUITES.push({
    name: 'postgres+redis',
    setup: async () => {
      const { getPool } = await import('../../adapters/persistence/postgres/pool')
      const { getRedis } = await import('../../adapters/persistence/redis/client')
      const pool = await getPool({ connectionString: dbUrl })
      const redis = await getRedis({ url: redisUrl })

      // テスト用にテーブルをクリーン (本番では絶対呼ばない)
      await pool.query(
        `TRUNCATE refresh_tokens, consents, users, clients, signing_keys RESTART IDENTITY CASCADE`,
      )
      await redis.flushdb()

      const { PostgresClientRepository } = await import(
        '../../adapters/persistence/postgres/client-repository'
      )
      const { PostgresUserRepository } = await import(
        '../../adapters/persistence/postgres/user-repository'
      )
      const { PostgresConsentRepository } = await import(
        '../../adapters/persistence/postgres/consent-repository'
      )
      const { PostgresRefreshTokenStore } = await import(
        '../../adapters/persistence/postgres/refresh-token-store'
      )
      const { RedisAuthorizationCodeStore, RedisPARStore } = await import(
        '../../adapters/persistence/redis/authorization-store'
      )
      const { RedisDpopReplayStore } = await import(
        '../../adapters/persistence/redis/dpop-replay-store'
      )
      const { PostgresKeyStore } = await import('../../adapters/persistence/postgres/key-store')

      return {
        clientRepo: new PostgresClientRepository(pool),
        userRepo: new PostgresUserRepository(pool),
        consentRepo: new PostgresConsentRepository(pool),
        refreshStore: new PostgresRefreshTokenStore(pool),
        codeStore: new RedisAuthorizationCodeStore(redis),
        parStore: new RedisPARStore(redis),
        dpopReplayStore: new RedisDpopReplayStore(redis),
        keyStore: await PostgresKeyStore.create(pool, 'PS256'),
      }
    },
    teardown: async () => {
      const { closePool } = await import('../../adapters/persistence/postgres/pool')
      const { closeRedis } = await import('../../adapters/persistence/redis/client')
      await closePool()
      await closeRedis()
    },
  })
}

for (const suite of SUITES) {
  describe(`persistence contract — ${suite.name}`, () => {
    let deps: Awaited<ReturnType<typeof suite.setup>>

    beforeAll(async () => {
      deps = await suite.setup()
    })

    if (suite.teardown) {
      afterAll(suite.teardown)
    }

    // -------------------------------------------------------------
    // ClientRepository
    // -------------------------------------------------------------
    describe('ClientRepository', () => {
      it('save → findById でラウンドトリップ', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        await deps.clientRepo.save(c)
        const found = await deps.clientRepo.findById(c.client_id)
        expect(found?.client_id).toBe(c.client_id)
        expect(found?.client_type).toBe('confidential')
      })

      it('未登録 client_id は null を返す', async () => {
        const found = await deps.clientRepo.findById('does-not-exist')
        expect(found).toBeNull()
      })
    })

    // -------------------------------------------------------------
    // UserRepository
    // -------------------------------------------------------------
    describe('UserRepository', () => {
      it('save → findBySub / findByUsername でラウンドトリップ', async () => {
        const u = makeUser(`u-${randomUUID()}`)
        await deps.userRepo.save(u)
        const bySub = await deps.userRepo.findBySub(u.sub)
        const byName = await deps.userRepo.findByUsername(u.preferred_username)
        expect(bySub?.sub).toBe(u.sub)
        expect(byName?.sub).toBe(u.sub)
      })
    })

    // -------------------------------------------------------------
    // ConsentRepository
    // -------------------------------------------------------------
    describe('ConsentRepository', () => {
      it('save → find → revoke', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        const u = makeUser(`u-${randomUUID()}`)
        await deps.clientRepo.save(c)
        await deps.userRepo.save(u)

        const con = makeConsent(u.sub, c.client_id)
        await deps.consentRepo.save(con)

        const found = await deps.consentRepo.find(u.sub, c.client_id)
        expect(found?.scopes).toEqual(['openid', 'profile'])

        await deps.consentRepo.revoke(u.sub, c.client_id)
        const afterRevoke = await deps.consentRepo.find(u.sub, c.client_id)
        expect(afterRevoke?.revoked_at).toBeDefined()
      })
    })

    // -------------------------------------------------------------
    // RefreshTokenStore — ADR-004 が要求する atomic rotate と family revoke
    // -------------------------------------------------------------
    describe('RefreshTokenStore', () => {
      it('save → findByHash でラウンドトリップ', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        await deps.clientRepo.save(c)
        const r = makeRefresh(c.client_id, 'sub-1')
        await deps.refreshStore.save(r)
        const found = await deps.refreshStore.findByHash(r.hash)
        expect(found?.id).toBe(r.id)
      })

      it('rotate は parent を rotated にし、新トークンを保存する', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        await deps.clientRepo.save(c)
        const parent = makeRefresh(c.client_id, 'sub-2')
        await deps.refreshStore.save(parent)

        const child = makeRefresh(c.client_id, 'sub-2', parent.family_id)
        child.parent_id = parent.id
        const result = await deps.refreshStore.rotate(parent.id, child)
        expect(result?.id).toBe(child.id)

        const parentAfter = await deps.refreshStore.findByHash(parent.hash)
        expect(parentAfter?.rotated).toBe(true)
      })

      it('rotated トークンを再度 rotate しようとしても null を返す (リプレイ検出)', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        await deps.clientRepo.save(c)
        const parent = makeRefresh(c.client_id, 'sub-3')
        await deps.refreshStore.save(parent)

        const child = makeRefresh(c.client_id, 'sub-3', parent.family_id)
        child.parent_id = parent.id
        await deps.refreshStore.rotate(parent.id, child)

        const replay = makeRefresh(c.client_id, 'sub-3', parent.family_id)
        replay.parent_id = parent.id
        const result = await deps.refreshStore.rotate(parent.id, replay)
        expect(result).toBeNull()
      })

      it('revokeFamily は family_id に紐付くすべてのトークンを revoked にする', async () => {
        const c = makeClient(`c-${randomUUID()}`)
        await deps.clientRepo.save(c)
        const r1 = makeRefresh(c.client_id, 'sub-4')
        const r2 = makeRefresh(c.client_id, 'sub-4', r1.family_id)
        const r3 = makeRefresh(c.client_id, 'sub-4', r1.family_id)
        await deps.refreshStore.save(r1)
        await deps.refreshStore.save(r2)
        await deps.refreshStore.save(r3)

        await deps.refreshStore.revokeFamily(r1.family_id)

        for (const r of [r1, r2, r3]) {
          const after = await deps.refreshStore.findByHash(r.hash)
          expect(after?.revoked).toBe(true)
        }
      })
    })

    // -------------------------------------------------------------
    // AuthorizationCodeStore — RFC 9700 §4.10 atomic redeem
    // -------------------------------------------------------------
    describe('AuthorizationCodeStore', () => {
      it('save → find → redeem (1st OK, 2nd null)', async () => {
        const code = makeAuthCode('c-redeem', 'sub-rc')
        await deps.codeStore.save(code)

        const first = await deps.codeStore.redeem(code.code)
        expect(first?.redeemed_at).toBeDefined()

        const second = await deps.codeStore.redeem(code.code)
        expect(second).toBeNull()
      })

      it('linkFamily で issued_family_id を後付け可能', async () => {
        const code = makeAuthCode('c-link', 'sub-rl')
        await deps.codeStore.save(code)
        await deps.codeStore.redeem(code.code)

        const family_id = randomUUID()
        await deps.codeStore.linkFamily(code.code, family_id)
        const after = await deps.codeStore.find(code.code)
        expect(after?.issued_family_id).toBe(family_id)
      })
    })

    // -------------------------------------------------------------
    // PARStore
    // -------------------------------------------------------------
    describe('PARStore', () => {
      it('save → consume (1st OK, 2nd null)', async () => {
        const par = makePAR('c-par')
        await deps.parStore.save(par)
        const first = await deps.parStore.consume(par.request_uri)
        expect(first?.used).toBe(true)
        const second = await deps.parStore.consume(par.request_uri)
        expect(second).toBeNull()
      })
    })

    // -------------------------------------------------------------
    // DpopReplayStore — リプレイ検出
    // -------------------------------------------------------------
    describe('DpopReplayStore', () => {
      it('同じ jti は 1 回のみ true を返す', async () => {
        const jti = `jti-${randomUUID()}`
        const first = await deps.dpopReplayStore.recordIfNew(jti, 600)
        const second = await deps.dpopReplayStore.recordIfNew(jti, 600)
        expect(first).toBe(true)
        expect(second).toBe(false)
      })
    })

    // -------------------------------------------------------------
    // KeyStore — ADR-009 durable + shared signing keys
    // -------------------------------------------------------------
    describe('KeyStore', () => {
      it('起動時に active 鍵が 1 つ存在し、PS256/ES256 である', async () => {
        const active = await deps.keyStore.getActiveKey()
        expect(active.kid.length).toBeGreaterThan(0)
        expect(['PS256', 'ES256']).toContain(active.alg)
        const byKid = await deps.keyStore.findByKid(active.kid)
        expect(byKid?.kid).toBe(active.kid)
      })

      it('rotate は新 active 鍵を作り、旧鍵を検証用に残す (オーバーラップ)', async () => {
        const before = await deps.keyStore.getActiveKey()
        const rotated = await deps.keyStore.rotate()
        expect(rotated.kid).not.toBe(before.kid)
        expect(rotated.active).toBe(true)

        // 新 active 鍵が getActiveKey で返る
        const after = await deps.keyStore.getActiveKey()
        expect(after.kid).toBe(rotated.kid)

        // 旧鍵は inactive で残り、findByKid / JWKS で引ける
        const old = await deps.keyStore.findByKid(before.kid)
        expect(old).not.toBeNull()
        expect(old?.active).toBe(false)
        const jwks = await deps.keyStore.getAllKeys()
        const kids = jwks.map((k) => k.kid)
        expect(kids).toContain(before.kid)
        expect(kids).toContain(rotated.kid)
      })

      it('active 鍵は常に高々 1 つ (single-active 不変条件)', async () => {
        await deps.keyStore.rotate()
        await deps.keyStore.rotate()
        const all = await deps.keyStore.getAllKeys()
        const activeCount = all.filter((k) => k.active).length
        expect(activeCount).toBe(1)
      })
    })
  })
}
