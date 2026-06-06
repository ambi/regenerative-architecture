/**
 * Layer 5 — Runtime & Infrastructure
 *
 * エントリーポイント。依存性の組み立てのみを行う。
 * ここを差し替えることで、ランタイム (Node / Bun / Deno / Lambda) や
 * 永続化 (InMemory → Postgres / Redis) を切り替える。
 *
 * 仕様核 (spec/)・アプリケーション層 (src/)・HTTP アダプタ (adapters/http/) は
 * 環境変数によって一切影響を受けない。これが ADR-003 / ADR-016 の実コード証明。
 *
 * 環境変数:
 *   PORT                  3000
 *   ISSUER                http://localhost:${PORT}
 *   PERSISTENCE           memory | postgres            (default: memory)
 *   EVENT_SINK            console | outbox             (default: console)
 *   OBSERVABILITY         noop | otel                  (default: noop)
 *   DATABASE_URL          postgres://...               (PERSISTENCE=postgres or EVENT_SINK=outbox 時に必須)
 *   REDIS_URL             redis://...                  (PERSISTENCE=postgres 時に必須)
 *   OTEL_EXPORTER_OTLP_ENDPOINT  http://...:4318       (OBSERVABILITY=otel 時に推奨)
 *   DEMO_CLIENT_SECRET    任意                          (デモクライアント用)
 *   SKIP_DEMO_SEED        any                          (本番起動時の seed スキップ)
 */

import { Hono } from 'hono'
import { createHash } from 'crypto'

import { InMemoryClientRepository } from './adapters/persistence/memory/client-repo'
import { InMemoryUserRepository } from './adapters/persistence/memory/user-repo'
import { InMemoryConsentRepository } from './adapters/persistence/memory/consent-repo'
import {
  InMemoryAuthorizationRequestStore,
  InMemoryAuthorizationCodeStore,
  InMemoryPARStore,
} from './adapters/persistence/memory/authorization-store'
import { InMemoryRefreshTokenStore } from './adapters/persistence/memory/refresh-store'
import { InMemoryDpopReplayStore } from './adapters/persistence/memory/dpop-replay-store'
import { InMemoryClientAssertionReplayStore } from './adapters/persistence/memory/client-assertion-replay-store'
import { InMemoryDeviceCodeStore } from './adapters/persistence/memory/device-code-store'
import { InMemorySessionStore } from './adapters/persistence/memory/session-store'
import { InMemoryKeyStore } from './adapters/crypto/in-memory-key-store'
import { JoseTokenSigner } from './adapters/crypto/jwt-signer'

import { ConsoleEventSink } from './adapters/event-sink/console'
import { NoopObserver } from './adapters/observability/noop'
import { createObservabilityMiddleware } from './adapters/http/middleware/observability-middleware'

import { createDiscoveryRoutes } from './adapters/http/discovery-routes'
import { createRegistrationRoutes } from './adapters/http/registration-routes'
import {
  createAuthorizationLoginContinuation,
  createAuthorizeRoutes,
} from './adapters/http/authorize-routes'
import { createAuthenticationRoutes } from './adapters/http/authentication-routes'
import { createTokenRoutes } from './adapters/http/token-routes'
import { createPARRoutes } from './adapters/http/par-routes'
import { createIntrospectionRoutes } from './adapters/http/introspection-routes'
import { createUserInfoRoutes } from './adapters/http/userinfo-routes'
import { createDeviceRoutes } from './adapters/http/device-routes'
import { createHealthRoutes } from './adapters/http/health-routes'
import { createEventsRoutes } from './adapters/http/events-routes'

import { ClientSchema, UserSchema, type DomainEvent } from './src/spec-bindings/schemas'
import type { ClientRepository } from './src/oauth2/ports/client-repository'
import type { UserRepository } from './src/authentication/ports/user-repository'
import type { ConsentRepository } from './src/oauth2/ports/consent-repository'
import type { RefreshTokenStore } from './src/oauth2/ports/refresh-token-store'
import type {
  AuthorizationRequestStore,
  AuthorizationCodeStore,
  PARStore,
} from './src/oauth2/ports/authorization-store'
import type { DpopReplayStore } from './src/oauth2/ports/dpop-replay-store'
import type { ClientAssertionReplayStore } from './src/oauth2/ports/client-assertion-replay-store'
import type { DeviceCodeStore } from './src/oauth2/ports/device-code-store'
import type { KeyStore } from './src/oauth2/ports/key-store'
import type { SessionStore } from './src/authentication/ports/session-store'
import { DEVICE_CODE_TTL_SECONDS } from './src/oauth2/domain/device-authorization'
import type { EventSink } from './src/shared/ports/event-sink'
import type { Observer } from './src/shared/ports/observer'
import { Argon2idPasswordHasher } from './adapters/crypto/argon2id-password-hasher'
import {
  PasswordPolicyError,
  validatePassword,
} from './src/authentication/usecases/password-policy'
import { LoginSessionManager } from './src/authentication/usecases/session-manager'
import { DemoHeaderResolver } from './src/authentication/usecases/demo-header-resolver'
import type { AuthenticationContextResolver } from './src/authentication/domain/authentication-context'

// ---------------------------------------------------------------
// 設定
// ---------------------------------------------------------------
const port = Number(process.env.PORT ?? 3000)
const issuer = process.env.ISSUER ?? `http://localhost:${port}`
const persistenceMode = (process.env.PERSISTENCE ?? 'memory') as 'memory' | 'postgres'
const eventSinkMode = (process.env.EVENT_SINK ?? 'console') as 'console' | 'outbox'
const observabilityMode = (process.env.OBSERVABILITY ?? 'noop') as 'noop' | 'otel'

// ---------------------------------------------------------------
// 依存性の組み立て (DI 合成ルート)
//
// すべての分岐はここに閉じ込める。src/* と adapters/http/* は無変更。
// ---------------------------------------------------------------
interface AssembledDeps {
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
  eventSink: EventSink
  collectedConsoleEvents?: ConsoleEventSink
}

async function assemble(): Promise<AssembledDeps> {
  if (persistenceMode === 'memory') {
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
      eventSink: consoleSink,
      collectedConsoleEvents: consoleSink,
    }
  }

  // PERSISTENCE=postgres → Postgres + Redis (ADR-016)
  const dbUrl = process.env.DATABASE_URL
  const redisUrl = process.env.REDIS_URL
  if (!dbUrl) throw new Error('PERSISTENCE=postgres requires DATABASE_URL')
  if (!redisUrl) throw new Error('PERSISTENCE=postgres requires REDIS_URL')

  const { getPool } = await import('./adapters/persistence/postgres/pool')
  const { getRedis } = await import('./adapters/persistence/redis/client')
  const pool = await getPool({ connectionString: dbUrl })
  const redis = await getRedis({ url: redisUrl })

  const { PostgresClientRepository } = await import(
    './adapters/persistence/postgres/client-repository'
  )
  const { PostgresUserRepository } = await import('./adapters/persistence/postgres/user-repository')
  const { PostgresConsentRepository } = await import(
    './adapters/persistence/postgres/consent-repository'
  )
  const { PostgresRefreshTokenStore } = await import(
    './adapters/persistence/postgres/refresh-token-store'
  )
  const { PostgresOutboxEventSink } = await import(
    './adapters/persistence/postgres/outbox-event-sink'
  )
  const { RedisAuthorizationRequestStore, RedisAuthorizationCodeStore, RedisPARStore } =
    await import('./adapters/persistence/redis/authorization-store')
  const { RedisDpopReplayStore } = await import('./adapters/persistence/redis/dpop-replay-store')
  const { RedisClientAssertionReplayStore } = await import(
    './adapters/persistence/redis/client-assertion-replay-store'
  )
  const { RedisDeviceCodeStore } = await import('./adapters/persistence/redis/device-code-store')
  const { RedisSessionStore } = await import('./adapters/persistence/redis/session-store')
  const { PostgresKeyStore } = await import('./adapters/persistence/postgres/key-store')

  const eventSink: EventSink =
    eventSinkMode === 'outbox' ? new PostgresOutboxEventSink(pool) : new ConsoleEventSink()

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
    eventSink,
  }
}

const deps = await assemble()
const keyStore = deps.keyStore
const tokenSigner = new JoseTokenSigner(issuer, keyStore)
const sessionManager = new LoginSessionManager(deps.sessionStore)
const demoHeaderResolver = new DemoHeaderResolver(deps.userRepo)
const authenticationContextResolver: AuthenticationContextResolver = {
  async resolve(headers) {
    return (await sessionManager.resolve(headers)) ?? (await demoHeaderResolver.resolve(headers))
  },
}
const authorizeRouteDeps = {
  clientRepo: deps.clientRepo,
  consentRepo: deps.consentRepo,
  requestStore: deps.requestStore,
  codeStore: deps.codeStore,
  parStore: deps.parStore,
  authenticationContextResolver,
  sessionManager,
  emit,
}

// ---------------------------------------------------------------
// Observability (ADR-017)
// ---------------------------------------------------------------
async function assembleObserver(): Promise<Observer> {
  if (observabilityMode === 'otel') {
    const { OtelObserver } = await import('./adapters/observability/otel')
    return await OtelObserver.create({
      serviceName: process.env.OTEL_SERVICE_NAME ?? 'ra-idp',
      eventSink: deps.eventSink,
    })
  }
  return new NoopObserver(deps.eventSink)
}
const observer = await assembleObserver()

// EventSink ポートを emit クロージャに射影 (既存 createXRoutes の API を変えない)
function emit(event: DomainEvent): void {
  // fire-and-forget。失敗は内部でログに残す責務 (Phase 2 で構造化ログ化)。
  deps.eventSink.publish(event).catch((err) => {
    // eslint-disable-next-line no-console
    console.error('[event-sink] publish failed:', err)
  })
}

const passwordHasher = new Argon2idPasswordHasher()

// ---------------------------------------------------------------
// 初期データ (デモ用)
// 本番想定で SKIP_DEMO_SEED が設定されていればスキップ。
// ---------------------------------------------------------------
if (!process.env.SKIP_DEMO_SEED) {
  const demoClientSecret = process.env.DEMO_CLIENT_SECRET ?? 'demo-secret-please-rotate'
  const demoClient = ClientSchema.parse({
    client_id: 'demo-web-app',
    client_secret_hash: createHash('sha256').update(demoClientSecret).digest('hex'),
    client_name: 'Demo Web Application',
    client_type: 'confidential',
    redirect_uris: ['http://localhost:8080/callback', 'https://app.example.com/callback'],
    grant_types: [
      'authorization_code',
      'refresh_token',
      'client_credentials',
      'urn:ietf:params:oauth:grant-type:device_code',
    ],
    response_types: ['code'],
    token_endpoint_auth_method: 'client_secret_basic',
    scope: 'openid profile email offline_access',
    id_token_signed_response_alg: 'PS256',
    require_pushed_authorization_requests: false,
    dpop_bound_access_tokens: false,
    fapi_profile: 'none',
    created_at: new Date().toISOString(),
  })
  await deps.clientRepo.save(demoClient)

  const demoPassword = process.env.DEMO_USER_PASSWORD ?? 'alice-password'
  const policy = validatePassword(demoPassword)
  if (!policy.ok) throw new PasswordPolicyError(policy.violations)
  const demoUser = UserSchema.parse({
    sub: 'user_alice',
    preferred_username: 'alice',
    password_hash: await passwordHasher.hash(demoPassword),
    name: 'Alice Demo',
    given_name: 'Alice',
    family_name: 'Demo',
    email: 'alice@example.com',
    email_verified: true,
    mfa_enrolled: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })
  await deps.userRepo.save(demoUser)
}

// ---------------------------------------------------------------
// ルーティング
// ---------------------------------------------------------------
const app = new Hono()

// 観測性ミドルウェアを最初に挿入 (すべてのリクエストで span + metric が記録される)
app.use('*', createObservabilityMiddleware(observer))

app.route('/', createDiscoveryRoutes({ issuer, keyStore }))
app.route('/', createRegistrationRoutes({ clientRepo: deps.clientRepo, emit }))
app.route('/', createAuthorizeRoutes(authorizeRouteDeps))
app.route(
  '/',
  createAuthenticationRoutes({
    userRepo: deps.userRepo,
    passwordHasher,
    sessionManager,
    continuation: createAuthorizationLoginContinuation(authorizeRouteDeps),
    emit,
  }),
)
app.route(
  '/',
  createTokenRoutes({
    issuer,
    clientRepo: deps.clientRepo,
    userRepo: deps.userRepo,
    codeStore: deps.codeStore,
    refreshStore: deps.refreshStore,
    deviceCodeStore: deps.deviceCodeStore,
    tokenIssuer: tokenSigner,
    dpopReplayStore: deps.dpopReplayStore,
    clientAssertionReplayStore: deps.clientAssertionReplayStore,
    emit,
  }),
)
app.route(
  '/',
  createDeviceRoutes({
    issuer,
    clientRepo: deps.clientRepo,
    userRepo: deps.userRepo,
    deviceCodeStore: deps.deviceCodeStore,
    clientAssertionReplayStore: deps.clientAssertionReplayStore,
    emit,
  }),
)
app.route(
  '/',
  createPARRoutes({
    issuer,
    clientRepo: deps.clientRepo,
    parStore: deps.parStore,
    clientAssertionReplayStore: deps.clientAssertionReplayStore,
    emit,
  }),
)
app.route(
  '/',
  createIntrospectionRoutes({
    issuer,
    clientRepo: deps.clientRepo,
    refreshStore: deps.refreshStore,
    introspector: tokenSigner,
    clientAssertionReplayStore: deps.clientAssertionReplayStore,
    emit,
  }),
)
app.route('/', createUserInfoRoutes({ introspector: tokenSigner, userRepo: deps.userRepo }))

// 運用補助
app.route(
  '/',
  createHealthRoutes({
    issuer,
    healthInfo: {
      persistence: persistenceMode,
      event_sink: eventSinkMode,
      observability: observabilityMode,
    },
  }),
)
if (deps.collectedConsoleEvents) {
  app.route('/', createEventsRoutes({ collectedEvents: deps.collectedConsoleEvents }))
}

app.notFound((c) => c.json({ error: 'not_found', error_description: c.req.path }, 404))

// Graceful shutdown: observer / pool / redis を flush
async function shutdown(signal: string): Promise<void> {
  // eslint-disable-next-line no-console
  console.log(`[main] received ${signal}, shutting down...`)
  try {
    await observer.shutdown()
  } catch {
    /* noop */
  }
  if (persistenceMode === 'postgres') {
    try {
      const { closePool } = await import('./adapters/persistence/postgres/pool')
      await closePool()
    } catch {
      /* noop */
    }
    try {
      const { closeRedis } = await import('./adapters/persistence/redis/client')
      await closeRedis()
    } catch {
      /* noop */
    }
  }
  process.exit(0)
}
process.on('SIGTERM', () => shutdown('SIGTERM'))
process.on('SIGINT', () => shutdown('SIGINT'))

// eslint-disable-next-line no-console
console.log(`\nOAuth2 / OIDC IdP — ${issuer}`)
console.log(
  `persistence=${persistenceMode}  event_sink=${eventSinkMode}  observability=${observabilityMode}`,
)
console.log('\n主要エンドポイント:')
console.log(`  GET    /.well-known/openid-configuration  Discovery (OIDC)`)
console.log(`  GET    /.well-known/oauth-authorization-server  Discovery (OAuth2)`)
console.log(`  GET    /jwks                              公開鍵 (JWKS)`)
console.log(`  POST   /register                          クライアント登録`)
console.log(`  POST   /device_authorization              デバイス認可リクエスト (RFC 8628)`)
console.log(`  GET    /device                            デバイス認可 verification_uri`)
console.log(`  GET    /authorize                         認可エンドポイント`)
console.log(`  POST   /login                             パスワードログイン`)
console.log(`  GET    /end_session                       RP-Initiated Logout`)
console.log(`  POST   /par                               Pushed Authorization Request`)
console.log(`  POST   /token                             トークンエンドポイント`)
console.log(`  POST   /introspect                        トークン introspection`)
console.log(`  POST   /revoke                            トークン失効`)
console.log(`  GET    /userinfo                          UserInfo (OIDC)`)
console.log(`  GET    /health                            ヘルスチェック`)
console.log(`  GET    /events                            イベント履歴 (memory モードのみ)`)
console.log(`\nデモ:`)
console.log(`  client_id     = demo-web-app`)
console.log(`  client_secret = ${process.env.DEMO_CLIENT_SECRET ?? 'demo-secret-please-rotate'}`)
console.log(`  user          = alice (X-User-Sub: user_alice)`)
console.log(`\nテストドライバー: ./demo.sh`)

export default {
  port,
  fetch: app.fetch,
}
