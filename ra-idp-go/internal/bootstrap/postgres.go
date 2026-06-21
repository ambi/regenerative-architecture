package bootstrap

import (
	"context"
	"errors"
	"os"

	"ra-idp-go/internal/adapters/persistence/postgres"
	redisstore "ra-idp-go/internal/adapters/persistence/redis"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/platform/eventsink"
)

func assemblePostgres(ctx context.Context) (*Dependencies, error) {
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
	return &Dependencies{
		TenantRepo:              &postgres.TenantRepository{Pool: pool},
		AttrSchemaRepo:          &postgres.TenantUserAttributeSchemaRepository{Pool: pool},
		ClientRepo:              &postgres.ClientRepository{Pool: pool},
		UserRepo:                &postgres.UserRepository{Pool: pool},
		GroupRepo:               &postgres.GroupRepository{Pool: pool},
		MfaFactorRepo:           &postgres.MfaFactorRepository{Pool: pool},
		PasswordHistoryRepo:     &postgres.PasswordHistoryRepository{Pool: pool},
		PasswordResetTokenStore: &postgres.PasswordResetTokenStore{Pool: pool},
		EmailChangeTokenStore:   &postgres.EmailChangeTokenStore{Pool: pool},
		ConsentRepo:             &postgres.ConsentRepository{Pool: pool},
		RequestStore:            &redisstore.AuthorizationRequestStore{Client: redisClient},
		CodeStore:               &redisstore.AuthorizationCodeStore{Client: redisClient},
		PARStore:                &redisstore.PARStore{Client: redisClient},
		RefreshStore:            &postgres.RefreshTokenStore{Pool: pool},
		DeviceCodeStore:         &redisstore.DeviceCodeStore{Client: redisClient},
		DpopReplay:              &redisstore.ReplayStore{Client: redisClient, Prefix: "dpop_replay:"},
		ClientAssertionReplay:   &redisstore.ReplayStore{Client: redisClient, Prefix: "client_assertion:"},
		AccessTokenDenylist:     &redisstore.AccessTokenDenylist{Client: redisClient},
		SessionStore:            &redisstore.SessionStore{Client: redisClient},
		KeyStore:                keyStore,
		EventSink:               sink,
		AuditEventRepo:          &postgres.AuditEventRepository{Pool: pool},
		AuthEventBucketStore:    &postgres.AuthEventBucketStore{Pool: pool},
		Close: func() {
			_ = redisClient.Close()
			pool.Close()
		},
	}, nil
}
