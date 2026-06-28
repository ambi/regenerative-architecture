package bootstrap

import (
	"context"
	"errors"
	"os"

	"ra-idp-go/internal/infrastructure/eventsink"
	"ra-idp-go/internal/infrastructure/persistence/postgres"
	valkeystore "ra-idp-go/internal/infrastructure/persistence/valkey"
	oauthports "ra-idp-go/internal/oauth2/ports"
)

func assemblePostgres(ctx context.Context) (*Dependencies, error) {
	databaseURL, valkeyURL := os.Getenv("DATABASE_URL"), os.Getenv("VALKEY_URL")
	if databaseURL == "" || valkeyURL == "" {
		return nil, errors.New("PERSISTENCE=postgres requires DATABASE_URL and VALKEY_URL")
	}
	pool, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if envDefault("AUTO_MIGRATE", "true") == "true" {
		if err := postgres.Migrate(ctx, pool, envDefault("MIGRATIONS_DIR", "deploy/migrations")); err != nil {
			pool.Close()
			return nil, err
		}
	}
	valkeyClient, err := valkeystore.Open(ctx, valkeyURL)
	if err != nil {
		pool.Close()
		return nil, err
	}
	keyStore, err := postgres.NewKeyStore(ctx, pool)
	if err != nil {
		pool.Close()
		_ = valkeyClient.Close()
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
		_ = valkeyClient.Close()
		return nil, errors.New("EVENT_SINK must be console or outbox")
	}
	return &Dependencies{
		TenantRepo:                &postgres.TenantRepository{Pool: pool},
		AttrSchemaRepo:            &postgres.TenantUserAttributeSchemaRepository{Pool: pool},
		ClientRepo:                &postgres.OAuth2ClientRepository{Pool: pool},
		UserRepo:                  &postgres.UserRepository{Pool: pool},
		GroupRepo:                 &postgres.GroupRepository{Pool: pool},
		AgentRepo:                 &postgres.AgentRepository{Pool: pool},
		MfaFactorRepo:             &postgres.MfaFactorRepository{Pool: pool},
		PasswordHistoryRepo:       &postgres.PasswordHistoryRepository{Pool: pool},
		PasswordResetTokenStore:   &postgres.PasswordResetTokenStore{Pool: pool},
		EmailChangeTokenStore:     &postgres.EmailChangeTokenStore{Pool: pool},
		ConsentRepo:               &postgres.ConsentRepository{Pool: pool},
		AuthzDetailTypeRepo:       &postgres.AuthorizationDetailTypeRepository{Pool: pool},
		RequestStore:              &valkeystore.AuthorizationRequestStore{Client: valkeyClient},
		CodeStore:                 &valkeystore.AuthorizationCodeStore{Client: valkeyClient},
		PARStore:                  &valkeystore.PARStore{Client: valkeyClient},
		RefreshStore:              &postgres.RefreshTokenStore{Pool: pool},
		DeviceCodeStore:           &valkeystore.DeviceCodeStore{Client: valkeyClient},
		DpopReplay:                &valkeystore.ReplayStore{Client: valkeyClient, Prefix: "dpop_replay:"},
		ClientAssertionReplay:     &valkeystore.ReplayStore{Client: valkeyClient, Prefix: "client_assertion:"},
		AccessTokenDenylist:       &valkeystore.AccessTokenDenylist{Client: valkeyClient},
		SessionStore:              &valkeystore.SessionStore{Client: valkeyClient},
		KeyStore:                  keyStore,
		EventSink:                 sink,
		AuditEventRepo:            &postgres.AuditEventRepository{Pool: pool},
		AuthEventBucketStore:      &postgres.AuthEventBucketStore{Pool: pool},
		WsFedRPRepo:               &postgres.WsFedRelyingPartyRepository{Pool: pool},
		SamlSPRepo:                &postgres.SamlServiceProviderRepository{Pool: pool},
		ApplicationRepo:           &postgres.ApplicationRepository{Pool: pool},
		ApplicationAssignmentRepo: &postgres.ApplicationAssignmentRepository{Pool: pool},
		ApplicationOrderingRepo:   &postgres.ApplicationOrderingRepository{Pool: pool},
		Close: func() {
			_ = valkeyClient.Close()
			pool.Close()
		},
	}, nil
}
