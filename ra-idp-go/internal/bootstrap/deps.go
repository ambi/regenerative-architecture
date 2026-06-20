package bootstrap

import (
	"context"
	"errors"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

// Dependencies は HTTP 層に渡す全境界をまとめた DI コンテナ。
// 永続層 (memory/postgres) や event sink の差分を本構造体で吸収する。
type Dependencies struct {
	ClientRepo              oauthports.ClientRepository
	TenantRepo              tenantports.TenantRepository
	AttrSchemaRepo          tenantports.TenantUserAttributeSchemaRepository
	UserRepo                oauthports.UserRepository
	GroupRepo               authports.GroupRepository
	MfaFactorRepo           authports.MfaFactorRepository
	PasswordHistoryRepo     authports.PasswordHistoryRepository
	PasswordResetTokenStore authports.PasswordResetTokenStore
	ConsentRepo             oauthports.ConsentRepository
	RequestStore            oauthports.AuthorizationRequestStore
	CodeStore               oauthports.AuthorizationCodeStore
	PARStore                oauthports.PARStore
	RefreshStore            oauthports.RefreshTokenStore
	DeviceCodeStore         oauthports.DeviceCodeStore
	DpopReplay              oauthports.DpopReplayStore
	ClientAssertionReplay   oauthports.ClientAssertionReplayStore
	AccessTokenDenylist     oauthports.AccessTokenDenylist
	SessionStore            authports.SessionStore
	KeyStore                oauthports.KeyStore
	EventSink               oauthports.EventSink
	AuditEventRepo          oauthports.AuditEventRepository
	Close                   func()
}

// RuntimeConfig は /health などで露出するための実行時構成ラベルを集約する。
type RuntimeConfig struct {
	Persistence   string
	EventSink     string
	Observability string
	AuthZEN       string
}

func loadRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Persistence:   envDefault("PERSISTENCE", "memory"),
		EventSink:     envDefault("EVENT_SINK", "console"),
		Observability: envDefault("OBSERVABILITY", "noop"),
		AuthZEN:       envDefault("AUTHZEN", "local"),
	}
}

// assemble は PERSISTENCE 環境変数に応じて memory/postgres いずれかの構成を組み立てる。
func assemble(ctx context.Context) (*Dependencies, error) {
	switch envDefault("PERSISTENCE", "memory") {
	case "memory":
		return assembleMemory()
	case "postgres":
		return assemblePostgres(ctx)
	default:
		return nil, errors.New("PERSISTENCE must be memory or postgres")
	}
}
