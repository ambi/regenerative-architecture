/**
 * Layer 5 — Runtime: Hono アプリの組み立て (ルートマウント)。
 *
 * すべての createXRoutes(deps) は本ファイルから呼ばれる。新しいエンドポイント
 * 群を増やすときはここに 1 行追加するだけで済む。
 */

import { Hono } from 'hono'

import { JoseTokenSigner } from '../adapters/crypto/jwt-signer'
import type { Argon2idPasswordHasher } from '../adapters/crypto/argon2id-password-hasher'
import { createObservabilityMiddleware } from '../adapters/http/middleware/observability-middleware'
import { createTenantMiddleware } from '../adapters/http/middleware/tenant-middleware'
import { createAdminUserRoutes } from '../adapters/http/admin-user-routes'
import { createAuthenticationRoutes } from '../adapters/http/authentication-routes'
import { createChangePasswordRoutes } from '../adapters/http/change-password-routes'
import { createPasswordResetRoutes } from '../adapters/http/password-reset-routes'
import {
  createAuthorizationLoginContinuation,
  createAuthorizeRoutes,
} from '../adapters/http/authorize-routes'
import { createDeviceRoutes } from '../adapters/http/device-routes'
import { createDiscoveryRoutes } from '../adapters/http/discovery-routes'
import { createEventsRoutes } from '../adapters/http/events-routes'
import { createHealthRoutes } from '../adapters/http/health-routes'
import { createIntrospectionRoutes } from '../adapters/http/introspection-routes'
import { createPARRoutes } from '../adapters/http/par-routes'
import { createRegistrationRoutes } from '../adapters/http/registration-routes'
import { createTokenRoutes } from '../adapters/http/token-routes'
import { createTotpRoutes } from '../adapters/http/totp-routes'
import { createUiAssetsRoutes } from '../adapters/http/ui-assets-routes'
import { createUserInfoRoutes } from '../adapters/http/userinfo-routes'

import type { AuthenticationContextResolver } from '../src/authentication/domain/authentication-context'
import { DemoHeaderResolver } from '../src/authentication/usecases/demo-header-resolver'
import { LoginSessionManager } from '../src/authentication/usecases/session-manager'
import type { Observer } from '../src/shared/ports/observer'
import type { DomainEvent } from '../src/spec-bindings/schemas'

import type { RuntimeConfig } from './config'
import type { AssembledDeps } from './dependencies'

export interface ComposeAppInput {
  config: RuntimeConfig
  deps: AssembledDeps
  observer: Observer
  passwordHasher: Argon2idPasswordHasher
  emit: (event: DomainEvent) => void
  sentinelPasswordHash: string
}

export function composeApp(input: ComposeAppInput): Hono {
  const { config, deps, observer, passwordHasher, emit, sentinelPasswordHash } = input

  const tokenSigner = new JoseTokenSigner(config.issuer, deps.keyStore)
  const sessionManager = new LoginSessionManager(deps.sessionStore, deps.userRepo)
  const demoHeaderResolver = new DemoHeaderResolver(deps.userRepo)
  const authenticationContextResolver: AuthenticationContextResolver = {
    async resolve(headers) {
      return (await sessionManager.resolve(headers)) ?? (await demoHeaderResolver.resolve(headers))
    },
  }
  const authorizeRouteDeps = {
    issuer: config.issuer,
    clientRepo: deps.clientRepo,
    consentRepo: deps.consentRepo,
    userRepo: deps.userRepo,
    requestStore: deps.requestStore,
    codeStore: deps.codeStore,
    parStore: deps.parStore,
    authenticationContextResolver,
    sessionManager,
    emit,
  }

  const app = new Hono()
  app.use('*', createObservabilityMiddleware(observer))
  // ADR-033: `/realms/{tenant_id}/...` または bare path のいずれでもテナントを
  // ctx に解決し、後段の route handler はテナント境界に従って動く。
  app.use(
    '*',
    createTenantMiddleware({
      tenantRepo: deps.tenantRepo,
      baseIssuer: config.issuer,
      legacyBareIssuer: config.legacyBareIssuer,
    }),
  )

  // プロトコル route 群を一度だけ組み立て、bare と /realms/:tenant_id の両方に
  // マウントする (ADR-033 §1)。Hono の subapp 再利用は副作用なく行える。
  const protocol = new Hono()
  protocol.route('/', createDiscoveryRoutes({ issuer: config.issuer, keyStore: deps.keyStore }))
  protocol.route('/', createRegistrationRoutes({ clientRepo: deps.clientRepo, emit }))
  protocol.route('/', createAuthorizeRoutes(authorizeRouteDeps))
  protocol.route(
    '/',
    createAuthenticationRoutes({
      userRepo: deps.userRepo,
      passwordHasher,
      sessionManager,
      continuation: createAuthorizationLoginContinuation(authorizeRouteDeps),
      emit,
      loginAttemptThrottle: deps.loginAttemptThrottle,
      sentinelPasswordHash,
      trustedForwardedHops: deps.trustedForwardedHops,
    }),
  )
  protocol.route(
    '/',
    createTotpRoutes({
      sessionManager,
      mfaFactorRepo: deps.mfaFactorRepo,
      continuation: createAuthorizationLoginContinuation(authorizeRouteDeps),
      emit,
    }),
  )
  protocol.route(
    '/',
    createChangePasswordRoutes({
      sessionManager,
      userRepo: deps.userRepo,
      passwordHasher,
      passwordHistoryRepo: deps.passwordHistoryRepo,
      breachedPasswordChecker: deps.breachedPasswordChecker,
      emit,
    }),
  )
  protocol.route(
    '/',
    createAdminUserRoutes({
      sessionManager,
      userRepo: deps.userRepo,
      passwordHasher,
      passwordHistoryRepo: deps.passwordHistoryRepo,
      emit,
    }),
  )
  protocol.route(
    '/',
    createPasswordResetRoutes({
      userRepo: deps.userRepo,
      passwordHasher,
      passwordHistoryRepo: deps.passwordHistoryRepo,
      passwordResetTokenStore: deps.passwordResetTokenStore,
      emailSender: deps.emailSender,
      breachedPasswordChecker: deps.breachedPasswordChecker,
      emit,
      issuer: config.issuer,
    }),
  )
  protocol.route(
    '/',
    createTokenRoutes({
      issuer: config.issuer,
      clientRepo: deps.clientRepo,
      userRepo: deps.userRepo,
      codeStore: deps.codeStore,
      refreshStore: deps.refreshStore,
      deviceCodeStore: deps.deviceCodeStore,
      tokenIssuer: tokenSigner,
      dpopReplayStore: deps.dpopReplayStore,
      dpopNonceService: deps.dpopNonceService,
      clientAssertionReplayStore: deps.clientAssertionReplayStore,
      emit,
    }),
  )
  protocol.route(
    '/',
    createDeviceRoutes({
      issuer: config.issuer,
      clientRepo: deps.clientRepo,
      userRepo: deps.userRepo,
      deviceCodeStore: deps.deviceCodeStore,
      clientAssertionReplayStore: deps.clientAssertionReplayStore,
      emit,
    }),
  )
  protocol.route(
    '/',
    createPARRoutes({
      issuer: config.issuer,
      clientRepo: deps.clientRepo,
      parStore: deps.parStore,
      clientAssertionReplayStore: deps.clientAssertionReplayStore,
      emit,
    }),
  )
  protocol.route(
    '/',
    createIntrospectionRoutes({
      issuer: config.issuer,
      clientRepo: deps.clientRepo,
      refreshStore: deps.refreshStore,
      introspector: tokenSigner,
      accessTokenDenylist: deps.accessTokenDenylist,
      clientAssertionReplayStore: deps.clientAssertionReplayStore,
      emit,
    }),
  )
  protocol.route(
    '/',
    createUserInfoRoutes({
      issuer: config.issuer,
      introspector: tokenSigner,
      userRepo: deps.userRepo,
      dpopReplayStore: deps.dpopReplayStore,
      dpopNonceService: deps.dpopNonceService,
      accessTokenDenylist: deps.accessTokenDenylist,
    }),
  )

  app.route('/', createUiAssetsRoutes())
  app.route('/', protocol)
  app.route('/realms/:tenant_id', protocol)

  app.route(
    '/',
    createHealthRoutes({
      issuer: config.issuer,
      healthInfo: {
        persistence: config.persistenceMode,
        event_sink: config.eventSinkMode,
        observability: config.observabilityMode,
      },
    }),
  )
  if (deps.collectedConsoleEvents) {
    app.route('/', createEventsRoutes({ collectedEvents: deps.collectedConsoleEvents }))
  }

  app.notFound((c) => c.json({ error: 'not_found', error_description: c.req.path }, 404))
  return app
}
