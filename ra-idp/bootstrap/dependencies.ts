/**
 * Layer 5 — Runtime: 依存性の組み立て (DI 合成ルート)。
 *
 * すべての persistence / event-sink の分岐はここに閉じる。src/* と adapters/http/*
 * は本ファイルを知らない。postgres / redis モジュールは memory モード時に読み込まない
 * よう動的 import を維持する (ADR-016)。
 */

import { InMemoryClientRepository } from '../adapters/persistence/memory/client-repo'
import { InMemoryUserRepository } from '../adapters/persistence/memory/user-repo'
import { InMemoryConsentRepository } from '../adapters/persistence/memory/consent-repo'
import {
  InMemoryAuthorizationRequestStore,
  InMemoryAuthorizationCodeStore,
  InMemoryPARStore,
} from '../adapters/persistence/memory/authorization-store'
import { InMemoryRefreshTokenStore } from '../adapters/persistence/memory/refresh-store'
import { InMemoryDpopReplayStore } from '../adapters/persistence/memory/dpop-replay-store'
import { InMemoryAccessTokenDenylist } from '../adapters/persistence/memory/access-token-denylist'
import { HmacDpopNonceService } from '../adapters/crypto/hmac-dpop-nonce-service'
import { InMemoryClientAssertionReplayStore } from '../adapters/persistence/memory/client-assertion-replay-store'
import { InMemoryDeviceCodeStore } from '../adapters/persistence/memory/device-code-store'
import { InMemorySessionStore } from '../adapters/persistence/memory/session-store'
import { InMemoryKeyStore } from '../adapters/crypto/in-memory-key-store'
import { ConsoleEventSink } from '../adapters/event-sink/console'

import type { AccessTokenDenylist } from '../src/oauth2/ports/access-token-denylist'
import type { DpopNonceService } from '../src/oauth2/ports/dpop-nonce-service'
import type { ClientRepository } from '../src/oauth2/ports/client-repository'
import type { UserRepository } from '../src/authentication/ports/user-repository'
import type { ConsentRepository } from '../src/oauth2/ports/consent-repository'
import type { RefreshTokenStore } from '../src/oauth2/ports/refresh-token-store'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from '../src/oauth2/ports/authorization-store'
import type { DpopReplayStore } from '../src/oauth2/ports/dpop-replay-store'
import type { ClientAssertionReplayStore } from '../src/oauth2/ports/client-assertion-replay-store'
import type { DeviceCodeStore } from '../src/oauth2/ports/device-code-store'
import type { KeyStore } from '../src/oauth2/ports/key-store'
import type { SessionStore } from '../src/authentication/ports/session-store'
import type { EventSink } from '../src/shared/ports/event-sink'
import { DEVICE_CODE_TTL_SECONDS } from '../src/oauth2/domain/device-authorization'

import type { RuntimeConfig } from './config'

export interface AssembledDeps {
  clientRepo: ClientRepository
  userRepo: UserRepository
  consentRepo: ConsentRepository
  requestStore: AuthorizationRequestStore
  codeStore: AuthorizationCodeStore
  parStore: PARStore
  refreshStore: RefreshTokenStore
  dpopReplayStore: DpopReplayStore
  clientAssertionReplayStore: ClientAssertionReplayStore
  deviceCodeStore: DeviceCodeStore
  sessionStore: SessionStore
  keyStore: KeyStore
  accessTokenDenylist: AccessTokenDenylist
  dpopNonceService: DpopNonceService
  eventSink: EventSink
  collectedConsoleEvents?: ConsoleEventSink
}

const DPOP_NONCE_TTL_SECONDS = 60

export async function assemble(config: RuntimeConfig): Promise<AssembledDeps> {
  if (config.persistenceMode === 'memory') {
    const consoleSink = new ConsoleEventSink({ collect: true })
    return {
      clientRepo: new InMemoryClientRepository(),
      userRepo: new InMemoryUserRepository(),
      consentRepo: new InMemoryConsentRepository(),
      requestStore: new InMemoryAuthorizationRequestStore(),
      codeStore: new InMemoryAuthorizationCodeStore(),
      parStore: new InMemoryPARStore(),
      refreshStore: new InMemoryRefreshTokenStore(),
      dpopReplayStore: new InMemoryDpopReplayStore(),
      clientAssertionReplayStore: new InMemoryClientAssertionReplayStore(),
      deviceCodeStore: new InMemoryDeviceCodeStore(),
      sessionStore: new InMemorySessionStore(),
      keyStore: await InMemoryKeyStore.create('PS256'),
      accessTokenDenylist: new InMemoryAccessTokenDenylist(),
      dpopNonceService: HmacDpopNonceService.withRandomSecret(DPOP_NONCE_TTL_SECONDS),
      eventSink: consoleSink,
      collectedConsoleEvents: consoleSink,
    }
  }

  // PERSISTENCE=postgres → Postgres + Redis (ADR-016)
  const dbUrl = process.env.DATABASE_URL
  const redisUrl = process.env.REDIS_URL
  if (!dbUrl) throw new Error('PERSISTENCE=postgres requires DATABASE_URL')
  if (!redisUrl) throw new Error('PERSISTENCE=postgres requires REDIS_URL')

  const { getPool } = await import('../adapters/persistence/postgres/pool')
  const { getRedis } = await import('../adapters/persistence/redis/client')
  const pool = await getPool({ connectionString: dbUrl })
  const redis = await getRedis({ url: redisUrl })

  const { PostgresClientRepository } = await import(
    '../adapters/persistence/postgres/client-repository'
  )
  const { PostgresUserRepository } = await import('../adapters/persistence/postgres/user-repository')
  const { PostgresConsentRepository } = await import(
    '../adapters/persistence/postgres/consent-repository'
  )
  const { PostgresRefreshTokenStore } = await import(
    '../adapters/persistence/postgres/refresh-token-store'
  )
  const { PostgresOutboxEventSink } = await import(
    '../adapters/persistence/postgres/outbox-event-sink'
  )
  const { RedisAuthorizationRequestStore, RedisAuthorizationCodeStore, RedisPARStore } =
    await import('../adapters/persistence/redis/authorization-store')
  const { RedisDpopReplayStore } = await import('../adapters/persistence/redis/dpop-replay-store')
  const { RedisClientAssertionReplayStore } = await import(
    '../adapters/persistence/redis/client-assertion-replay-store'
  )
  const { RedisDeviceCodeStore } = await import('../adapters/persistence/redis/device-code-store')
  const { RedisSessionStore } = await import('../adapters/persistence/redis/session-store')
  const { PostgresKeyStore } = await import('../adapters/persistence/postgres/key-store')
  const { RedisAccessTokenDenylist } = await import(
    '../adapters/persistence/redis/access-token-denylist'
  )

  const eventSink: EventSink =
    config.eventSinkMode === 'outbox' ? new PostgresOutboxEventSink(pool) : new ConsoleEventSink()

  return {
    clientRepo: new PostgresClientRepository(pool),
    userRepo: new PostgresUserRepository(pool),
    consentRepo: new PostgresConsentRepository(pool),
    requestStore: new RedisAuthorizationRequestStore(redis),
    codeStore: new RedisAuthorizationCodeStore(redis),
    parStore: new RedisPARStore(redis),
    refreshStore: new PostgresRefreshTokenStore(pool),
    dpopReplayStore: new RedisDpopReplayStore(redis),
    clientAssertionReplayStore: new RedisClientAssertionReplayStore(redis),
    deviceCodeStore: new RedisDeviceCodeStore(redis, DEVICE_CODE_TTL_SECONDS),
    sessionStore: new RedisSessionStore(redis),
    keyStore: await PostgresKeyStore.create(pool, 'PS256'),
    accessTokenDenylist: new RedisAccessTokenDenylist(redis),
    dpopNonceService: makeDpopNonceService(),
    eventSink,
  }
}

function makeDpopNonceService(): DpopNonceService {
  // 複数インスタンス間で nonce を相互に受理するため共有秘密が要る。
  // DPOP_NONCE_SECRET 未設定時はランダム生成 (単一インスタンス前提)。
  const env = process.env.DPOP_NONCE_SECRET
  if (env && env.length >= 32) {
    return new HmacDpopNonceService(Buffer.from(env, 'utf8'), DPOP_NONCE_TTL_SECONDS)
  }
  return HmacDpopNonceService.withRandomSecret(DPOP_NONCE_TTL_SECONDS)
}
