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

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/shared/adapters/crypto"
	httpadapter "ra-idp-go/internal/shared/adapters/http/server"
	httpsupport "ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/adapters/observability"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"
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
		if err := seedDemoData(ctx, deps.ClientRepo, deps.UserRepo, deps.MfaFactorRepo, deps.PasswordHistoryRepo, deps.GroupRepo, deps.AuthzDetailTypeRepo, hasher); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
		if err := seedWsFedRelyingParty(ctx, deps.WsFedRPRepo); err != nil {
			return fmt.Errorf("seed federation relying party: %w", err)
		}
		if err := seedSamlServiceProvider(ctx, deps.SamlSPRepo); err != nil {
			return fmt.Errorf("seed saml service provider: %w", err)
		}
		if err := seedDemoApplications(ctx, deps.ApplicationRepo, deps.ApplicationAssignmentRepo, time.Now().UTC()); err != nil {
			return fmt.Errorf("seed demo applications: %w", err)
		}
	}
	federationSigner, err := newDevFederationSigner()
	if err != nil {
		return fmt.Errorf("federation signer: %w", err)
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
	breachedChecker, err := resolveBreachedPasswordChecker(os.Getenv)
	if err != nil {
		return fmt.Errorf("resolve breached password checker: %w", err)
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
	httpadapter.Register(e, httpsupport.Deps{
		Issuer: issuer, SCL: sclDoc,
		TenantRepo:       deps.TenantRepo,
		AttrSchemaRepo:   deps.AttrSchemaRepo,
		LegacyBareIssuer: envDefault("LEGACY_BARE_ISSUER", "false") == "true",
		ClientRepo:       deps.ClientRepo, UserRepo: deps.UserRepo, ConsentRepo: deps.ConsentRepo,
		AuthzDetailTypeRepo: deps.AuthzDetailTypeRepo,
		RequestStore:        deps.RequestStore, CodeStore: deps.CodeStore, PARStore: deps.PARStore,
		RefreshStore: deps.RefreshStore, DeviceCodeStore: deps.DeviceCodeStore,
		DpopReplayStore: deps.DpopReplay, ClientAssertionReplayStore: deps.ClientAssertionReplay,
		AccessTokenDenylist: deps.AccessTokenDenylist,
		KeyStore:            deps.KeyStore, TokenIssuer: tokenSigner, TokenIntrospector: tokenSigner,
		AuditEventRepo:       deps.AuditEventRepo,
		AuthEventBucketStore: deps.AuthEventBucketStore,
		Authorizer:           authorizer, JWKResolver: jwkResolver,
		PasswordHasher: hasher, GroupRepo: deps.GroupRepo, AgentRepo: deps.AgentRepo, MfaFactorRepo: deps.MfaFactorRepo, PasswordHistoryRepo: deps.PasswordHistoryRepo,
		PasswordResetTokenStore: deps.PasswordResetTokenStore,
		EmailChangeTokenStore:   deps.EmailChangeTokenStore,
		EmailSender:             emailSender,
		BreachedPasswordChecker: breachedChecker,
		LoginAttemptThrottle:    loginThrottle,
		TrustedForwardedHops:    envInt("TRUSTED_FORWARDED_HOPS", 0),
		SentinelPasswordHash:    sentinelPasswordHash,
		SessionManager:          sessionManager, AuthnResolver: sessionManager,
		WsFedRPRepo: deps.WsFedRPRepo, SamlSPRepo: deps.SamlSPRepo, FederationSigner: federationSigner,
		ApplicationRepo: deps.ApplicationRepo, ApplicationIconStore: deps.ApplicationIconStore,
		ApplicationAssignmentRepo: deps.ApplicationAssignmentRepo,
		ApplicationOrderingRepo:   deps.ApplicationOrderingRepo,
		ApplicationCategoryRepo:   deps.ApplicationCategoryRepo,
		Emit:                      emit,
		HealthInfo: httpsupport.HealthInfo{
			Persistence:   runtime.Persistence,
			EventSink:     runtime.EventSink,
			Observability: runtime.Observability,
			AuthZEN:       runtime.AuthZEN,
		},
	})

	startRetentionSweep(ctx, deps, envDuration("RETENTION_SWEEP_INTERVAL", time.Hour))

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
