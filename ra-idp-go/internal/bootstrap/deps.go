package bootstrap

import (
	"context"
	"errors"

	appports "ra-idp-go/internal/application/ports"
	authnports "ra-idp-go/internal/authentication/ports"
	idmports "ra-idp-go/internal/identitymanagement/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	samlports "ra-idp-go/internal/saml/ports"
	tenantports "ra-idp-go/internal/tenancy/ports"
	wsfederationports "ra-idp-go/internal/wsfederation/ports"
)

// Dependencies は HTTP 層に渡す全境界をまとめた DI コンテナ。
// 永続層 (memory/postgres) や event sink の差分を本構造体で吸収する。
type Dependencies struct {
	ClientRepo                oauthports.OAuth2ClientRepository
	TenantRepo                tenantports.TenantRepository
	AttrSchemaRepo            tenantports.TenantUserAttributeSchemaRepository
	UserRepo                  oauthports.UserRepository
	GroupRepo                 idmports.GroupRepository
	AgentRepo                 idmports.AgentRepository
	MfaFactorRepo             authnports.MfaFactorRepository
	PasswordHistoryRepo       authnports.PasswordHistoryRepository
	PasswordResetTokenStore   authnports.PasswordResetTokenStore
	EmailChangeTokenStore     authnports.EmailChangeTokenStore
	ConsentRepo               oauthports.ConsentRepository
	AuthzDetailTypeRepo       oauthports.AuthorizationDetailTypeRepository
	RequestStore              oauthports.AuthorizationRequestStore
	CodeStore                 oauthports.AuthorizationCodeStore
	PARStore                  oauthports.PARStore
	RefreshStore              oauthports.RefreshTokenStore
	DeviceCodeStore           oauthports.DeviceCodeStore
	DpopReplay                oauthports.DpopReplayStore
	ClientAssertionReplay     oauthports.ClientAssertionReplayStore
	AccessTokenDenylist       oauthports.AccessTokenDenylist
	SessionStore              authnports.SessionStore
	KeyStore                  oauthports.KeyStore
	EventSink                 oauthports.EventSink
	AuditEventRepo            oauthports.AuditEventRepository
	AuthEventBucketStore      authnports.AuthEventBucketStore
	WsFedRPRepo               wsfederationports.WsFedRelyingPartyRepository
	SamlSPRepo                samlports.SamlServiceProviderRepository
	ApplicationRepo           appports.ApplicationRepository
	ApplicationAssignmentRepo appports.AssignmentRepository
	ApplicationOrderingRepo   appports.ApplicationOrderingRepository
	ApplicationCategoryRepo   appports.ApplicationCategoryRepository
	Close                     func()
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
