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
import { createAuthenticationRoutes } from '../adapters/http/authentication-routes'
import { createChangePasswordRoutes } from '../adapters/http/change-password-routes'
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
  const sessionManager = new LoginSessionManager(deps.sessionStore)
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

  app.route('/', createUiAssetsRoutes())
  app.route('/', createDiscoveryRoutes({ issuer: config.issuer, keyStore: deps.keyStore }))
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
      loginAttemptThrottle: deps.loginAttemptThrottle,
      sentinelPasswordHash,
      trustedForwardedHops: deps.trustedForwardedHops,
    }),
  )
  app.route(
    '/',
    createTotpRoutes({
      sessionManager,
      mfaFactorRepo: deps.mfaFactorRepo,
      continuation: createAuthorizationLoginContinuation(authorizeRouteDeps),
      emit,
    }),
  )
  app.route(
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
  app.route(
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
  app.route(
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
  app.route(
    '/',
    createPARRoutes({
      issuer: config.issuer,
      clientRepo: deps.clientRepo,
      parStore: deps.parStore,
      clientAssertionReplayStore: deps.clientAssertionReplayStore,
      emit,
    }),
  )
  app.route(
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
  app.route(
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
