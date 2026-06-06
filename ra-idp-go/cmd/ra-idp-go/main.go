package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/eventsink"
	httpadapter "ra-idp-go/internal/adapters/http"
	"ra-idp-go/internal/adapters/observability"
	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/adapters/persistence/postgres"
	redisstore "ra-idp-go/internal/adapters/persistence/redis"
	"ra-idp-go/internal/adapters/policy"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type assembled struct {
	clientRepo            oauthports.ClientRepository
	userRepo              oauthports.UserRepository
	consentRepo           oauthports.ConsentRepository
	requestStore          oauthports.AuthorizationRequestStore
	codeStore             oauthports.AuthorizationCodeStore
	parStore              oauthports.PARStore
	refreshStore          oauthports.RefreshTokenStore
	deviceCodeStore       oauthports.DeviceCodeStore
	dpopReplay            oauthports.DpopReplayStore
	clientAssertionReplay oauthports.ClientAssertionReplayStore
	sessionStore          oauthports.SessionStore
	keyStore              oauthports.KeyStore
	eventSink             oauthports.EventSink
	close                 func()
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	issuer := envDefault("ISSUER", "http://localhost:8080")
	addr := envDefault("ADDR", ":8080")

	deps, err := assemble(context.Background())
	if err != nil {
		return fmt.Errorf("assemble dependencies: %w", err)
	}
	defer deps.close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	hasher := crypto.NewArgon2idPasswordHasher()
	if os.Getenv("SKIP_DEMO_SEED") == "" {
		if err := seedDemoData(ctx, deps.clientRepo, deps.userRepo, hasher); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}
	sclDoc, err := spec.LoadSCL()
	if err != nil {
		return fmt.Errorf("load SCL: %w", err)
	}
	authorizer, err := assembleAuthorizer()
	if err != nil {
		return err
	}
	sessionManager := authusecases.NewSessionManager(deps.sessionStore)
	tokenSigner := crypto.NewJWTSigner(issuer, deps.keyStore)
	jwkResolver := crypto.NewJWKResolver()

	e := echo.New()
	var otelProvider *observability.Provider
	if envDefault("OBSERVABILITY", "noop") == "otel" {
		otelProvider, err = observability.New(ctx, envDefault("OTEL_SERVICE_NAME", "ra-idp-go"), "0.3.0")
		if err != nil {
			return fmt.Errorf("initialize OpenTelemetry: %w", err)
		}
		e.Use(otelProvider.Middleware)
	}
	emit := func(event spec.DomainEvent) {
		eventCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := deps.eventSink.Emit(eventCtx, event); err != nil {
			log.Printf("event sink: %v", err)
		}
	}
	httpadapter.Register(e, httpadapter.Deps{
		Issuer: issuer, SCL: sclDoc,
		ClientRepo: deps.clientRepo, UserRepo: deps.userRepo, ConsentRepo: deps.consentRepo,
		RequestStore: deps.requestStore, CodeStore: deps.codeStore, PARStore: deps.parStore,
		RefreshStore: deps.refreshStore, DeviceCodeStore: deps.deviceCodeStore,
		DpopReplayStore: deps.dpopReplay, ClientAssertionReplayStore: deps.clientAssertionReplay,
		KeyStore: deps.keyStore, TokenIssuer: tokenSigner, TokenIntrospector: tokenSigner,
		Authorizer: authorizer, JWKResolver: jwkResolver,
		PasswordHasher: hasher, SessionManager: sessionManager, AuthnResolver: sessionManager,
		Emit: emit,
		HealthInfo: httpadapter.HealthInfo{
			Persistence:   envDefault("PERSISTENCE", "memory"),
			EventSink:     envDefault("EVENT_SINK", "console"),
			Observability: envDefault("OBSERVABILITY", "noop"),
			AuthZEN:       envDefault("AUTHZEN", "local"),
		},
	})

	log.Printf("ra-idp-go listening on %s (issuer=%s)", addr, issuer)
	startConfig := echo.StartConfig{Address: addr}
	if err := startConfig.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server: %v", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if otelProvider != nil {
		if err := otelProvider.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown OpenTelemetry: %v", err)
		}
	}
	return nil
}

func assemble(ctx context.Context) (*assembled, error) {
	switch envDefault("PERSISTENCE", "memory") {
	case "memory":
		return &assembled{
			clientRepo:   memory.NewClientRepository(),
			userRepo:     memory.NewUserRepository(),
			consentRepo:  memory.NewConsentRepository(),
			requestStore: memory.NewAuthorizationRequestStore(), codeStore: memory.NewAuthorizationCodeStore(),
			parStore: memory.NewPARStore(), refreshStore: memory.NewRefreshTokenStore(),
			deviceCodeStore: memory.NewDeviceCodeStore(), dpopReplay: memory.NewDpopReplayStore(),
			clientAssertionReplay: memory.NewClientAssertionReplayStore(),
			sessionStore:          memory.NewSessionStore(),
			keyStore:              mustMemoryKeyStore(),
			eventSink:             eventsink.NewConsoleSink(),
			close:                 func() {},
		}, nil
	case "postgres":
		return assemblePostgres(ctx)
	default:
		return nil, errors.New("PERSISTENCE must be memory or postgres")
	}
}

func assemblePostgres(ctx context.Context) (*assembled, error) {
	databaseURL, redisURL := os.Getenv("DATABASE_URL"), os.Getenv("REDIS_URL")
	if databaseURL == "" || redisURL == "" {
		return nil, errors.New("PERSISTENCE=postgres requires DATABASE_URL and REDIS_URL")
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if envDefault("AUTO_MIGRATE", "true") == "true" {
		if err := postgres.Migrate(ctx, pool, envDefault("MIGRATIONS_DIR", "infra/migrations")); err != nil {
			pool.Close()
			return nil, err
		}
	}
	redisClient, err := redisstore.Open(ctx, redisURL)
	if err != nil {
		pool.Close()
		return nil, err
	}
	keyStore, err := postgres.NewKeyStore(ctx, pool)
	if err != nil {
		pool.Close()
		_ = redisClient.Close()
		return nil, err
	}
	var sink oauthports.EventSink
	switch envDefault("EVENT_SINK", "console") {
	case "console":
		sink = eventsink.NewConsoleSink()
	case "outbox":
		sink = &postgres.OutboxEventSink{Pool: pool}
	default:
		pool.Close()
		_ = redisClient.Close()
		return nil, errors.New("EVENT_SINK must be console or outbox")
	}
	return &assembled{
		clientRepo:            &postgres.ClientRepository{Pool: pool},
		userRepo:              &postgres.UserRepository{Pool: pool},
		consentRepo:           &postgres.ConsentRepository{Pool: pool},
		requestStore:          &redisstore.AuthorizationRequestStore{Client: redisClient},
		codeStore:             &redisstore.AuthorizationCodeStore{Client: redisClient},
		parStore:              &redisstore.PARStore{Client: redisClient},
		refreshStore:          &postgres.RefreshTokenStore{Pool: pool},
		deviceCodeStore:       &redisstore.DeviceCodeStore{Client: redisClient},
		dpopReplay:            &redisstore.ReplayStore{Client: redisClient, Prefix: "idp:dpop:jti:"},
		clientAssertionReplay: &redisstore.ReplayStore{Client: redisClient, Prefix: "idp:cassert:jti:"},
		sessionStore:          &redisstore.SessionStore{Client: redisClient},
		keyStore:              keyStore, eventSink: sink,
		close: func() {
			_ = redisClient.Close()
			pool.Close()
		},
	}, nil
}

func assembleAuthorizer() (oauthports.Authorizer, error) {
	switch envDefault("AUTHZEN", "local") {
	case "local":
		return policy.Local{}, nil
	case "remote":
		endpoint := os.Getenv("AUTHZEN_URL")
		if endpoint == "" {
			return nil, errors.New("AUTHZEN=remote requires AUTHZEN_URL")
		}
		return policy.NewRemote(endpoint), nil
	default:
		return nil, errors.New("AUTHZEN must be local or remote")
	}
}

func mustMemoryKeyStore() oauthports.KeyStore {
	store, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		panic(err)
	}
	return store
}

func seedDemoData(
	ctx context.Context,
	clients oauthports.ClientRepository,
	users oauthports.UserRepository,
	hasher authports.PasswordHasher,
) error {
	secretHash := oauthdomain.HashClientSecret(envDefault("DEMO_CLIENT_SECRET", "demo-client-secret"))
	now := time.Now().UTC()
	if err := clients.Save(ctx, &spec.Client{
		ClientID: "demo-client", ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
		RedirectURIs: []string{"http://localhost:3000/callback"},
		GrantTypes: []spec.GrantType{
			spec.GrantAuthorizationCode, spec.GrantRefreshToken,
			spec.GrantClientCredentials, spec.GrantDeviceCode,
		},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic,
		Scope:                   "openid profile email offline_access", IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile: spec.FapiNone, CreatedAt: now,
	}); err != nil {
		return err
	}
	password := envDefault("DEMO_USER_PASSWORD", "demo-password-1234")
	if result := authusecases.ValidatePassword(password); !result.OK {
		return errors.New("DEMO_USER_PASSWORD violates password policy")
	}
	hash, err := hasher.Hash(password)
	if err != nil {
		return err
	}
	email := "alice@example.com"
	return users.Save(ctx, &spec.User{
		Sub: "user_alice", PreferredUsername: "alice", PasswordHash: hash,
		Email: &email, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
