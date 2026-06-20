package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	httpadapter "ra-idp-go/internal/adapters/http"
	"ra-idp-go/internal/adapters/observability"
	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/adapters/policy"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"
	tenantusecases "ra-idp-go/internal/tenancy/usecases"

	"github.com/labstack/echo/v5"
)

// Run はサーバ全体を起動する。SIGINT/SIGTERM で graceful shutdown。
func Run() error {
	runtime := loadRuntimeConfig()
	issuer := envDefault("ISSUER", "http://localhost:8080")
	addr := envDefault("ADDR", ":8080")

	deps, err := assemble(context.Background())
	if err != nil {
		return fmt.Errorf("assemble dependencies: %w", err)
	}
	defer deps.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	hasher := crypto.NewArgon2idPasswordHasher()
	if err := tenantusecases.EnsureDefault(ctx, deps.TenantRepo, time.Now().UTC()); err != nil {
		return fmt.Errorf("ensure default tenant: %w", err)
	}
	if os.Getenv("SKIP_DEMO_SEED") == "" {
		if err := seedDemoData(ctx, deps.ClientRepo, deps.UserRepo, deps.MfaFactorRepo, deps.PasswordHistoryRepo, deps.GroupRepo, hasher); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}
	sclDoc, err := spec.LoadSCL()
	if err != nil {
		return fmt.Errorf("load SCL: %w", err)
	}
	sentinelPasswordHash, err := hasher.Hash("ra-idp-invalid-user-password")
	if err != nil {
		return fmt.Errorf("create sentinel password hash: %w", err)
	}
	emailSender, err := resolveEmailSender(os.Getenv)
	if err != nil {
		return fmt.Errorf("resolve email sender: %w", err)
	}
	objectiveInt := func(group, key string) int {
		value, ok := sclDoc.ObjectiveNestedInt("LoginThrottlePolicy", group, key)
		if !ok {
			return 0
		}
		return value
	}
	loginThrottle := memory.NewLoginAttemptThrottle(memory.LoginThrottleConfigs{
		Account: memory.LoginThrottleConfig{
			MaxFailures:    objectiveInt("per_account", "max_failures"),
			WindowSeconds:  objectiveInt("per_account", "window_seconds"),
			LockoutSeconds: objectiveInt("per_account", "lockout_seconds"),
		},
		IP: memory.LoginThrottleConfig{
			MaxFailures:    objectiveInt("per_ip", "max_failures"),
			WindowSeconds:  objectiveInt("per_ip", "window_seconds"),
			LockoutSeconds: objectiveInt("per_ip", "lockout_seconds"),
		},
	})
	authorizer, err := assembleAuthorizer()
	if err != nil {
		return err
	}
	sessionManager := authusecases.NewSessionManager(deps.SessionStore)
	tokenSigner := crypto.NewJWTSigner(issuer, deps.KeyStore)
	jwkResolver := crypto.NewJWKResolver()

	e := echo.New()
	var otelProvider *observability.Provider
	if runtime.Observability == "otel" {
		otelProvider, err = observability.New(ctx, envDefault("OTEL_SERVICE_NAME", "ra-idp-go"), "0.3.0")
		if err != nil {
			return fmt.Errorf("initialize OpenTelemetry: %w", err)
		}
		e.Use(otelProvider.Middleware)
	}
	emit := func(event spec.DomainEvent) {
		eventCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := deps.EventSink.Emit(eventCtx, event); err != nil {
			log.Printf("event sink: %v", err)
		}
		if deps.AuditEventRepo != nil {
			if rec, err := newAuditEventRecord(event); err == nil {
				_ = deps.AuditEventRepo.Append(eventCtx, rec)
			}
		}
	}
	httpadapter.Register(e, httpadapter.Deps{
		Issuer: issuer, SCL: sclDoc,
		TenantRepo:       deps.TenantRepo,
		AttrSchemaRepo:   deps.AttrSchemaRepo,
		LegacyBareIssuer: envDefault("LEGACY_BARE_ISSUER", "false") == "true",
		ClientRepo:       deps.ClientRepo, UserRepo: deps.UserRepo, ConsentRepo: deps.ConsentRepo,
		RequestStore: deps.RequestStore, CodeStore: deps.CodeStore, PARStore: deps.PARStore,
		RefreshStore: deps.RefreshStore, DeviceCodeStore: deps.DeviceCodeStore,
		DpopReplayStore: deps.DpopReplay, ClientAssertionReplayStore: deps.ClientAssertionReplay,
		AccessTokenDenylist: deps.AccessTokenDenylist,
		KeyStore:            deps.KeyStore, TokenIssuer: tokenSigner, TokenIntrospector: tokenSigner,
		AuditEventRepo: deps.AuditEventRepo,
		Authorizer:     authorizer, JWKResolver: jwkResolver,
		PasswordHasher: hasher, GroupRepo: deps.GroupRepo, MfaFactorRepo: deps.MfaFactorRepo, PasswordHistoryRepo: deps.PasswordHistoryRepo,
		PasswordResetTokenStore: deps.PasswordResetTokenStore,
		EmailSender:             emailSender,
		BreachedPasswordChecker: policy.NoopBreachedPasswordChecker{},
		LoginAttemptThrottle:    loginThrottle,
		TrustedForwardedHops:    envInt("TRUSTED_FORWARDED_HOPS", 0),
		SentinelPasswordHash:    sentinelPasswordHash,
		SessionManager:          sessionManager, AuthnResolver: sessionManager,
		Emit: emit,
		HealthInfo: httpadapter.HealthInfo{
			Persistence:   runtime.Persistence,
			EventSink:     runtime.EventSink,
			Observability: runtime.Observability,
			AuthZEN:       runtime.AuthZEN,
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
