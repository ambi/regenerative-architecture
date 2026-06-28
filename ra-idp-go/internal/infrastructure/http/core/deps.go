// Package core: HTTP アダプタの共有基盤。
//
// 複数コンテキスト (tenancy / authentication / oauth2) の infrastructure/http が
// 共通で使う依存集約 (Deps)・テナント解決 middleware・横断ヘルパのみを置く。
// 各コンテキストの adapters/http は core を import し、router (infrastructure/http) が
// 各コンテキストの RegisterRoutes を集約する。core ← context ← router の一方向。
package core

import (
	appports "ra-idp-go/internal/application/ports"
	authdomain "ra-idp-go/internal/authentication/domain"
	authnports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	idmports "ra-idp-go/internal/identitymanagement/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/infrastructure/crypto"
	samlports "ra-idp-go/internal/saml/ports"
	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
	"ra-idp-go/internal/wsfederation/adapters/samltoken"
	wsfederationports "ra-idp-go/internal/wsfederation/ports"
)

// Deps は全 HTTP ハンドラが共有する依存集約。bootstrap が一様に配線する。
type Deps struct {
	Issuer                     string
	SCL                        *spec.SCL
	TenantRepo                 tenantports.TenantRepository
	AttrSchemaRepo             tenantports.TenantUserAttributeSchemaRepository
	LegacyBareIssuer           bool
	ClientRepo                 oauthports.ClientRepository
	UserRepo                   oauthports.UserRepository
	ConsentRepo                oauthports.ConsentRepository
	AuthzDetailTypeRepo        oauthports.AuthorizationDetailTypeRepository
	RequestStore               oauthports.AuthorizationRequestStore
	CodeStore                  oauthports.AuthorizationCodeStore
	PARStore                   oauthports.PARStore
	RefreshStore               oauthports.RefreshTokenStore
	DeviceCodeStore            oauthports.DeviceCodeStore
	DpopReplayStore            oauthports.DpopReplayStore
	ClientAssertionReplayStore oauthports.ClientAssertionReplayStore
	AccessTokenDenylist        oauthports.AccessTokenDenylist
	KeyStore                   oauthports.KeyStore
	TokenIssuer                oauthports.TokenIssuer
	TokenIntrospector          oauthports.TokenIntrospector
	AuditEventRepo             oauthports.AuditEventRepository
	AuthEventBucketStore       authnports.AuthEventBucketStore
	Authorizer                 oauthports.Authorizer
	JWKResolver                *crypto.JWKResolver
	PasswordHasher             authnports.PasswordHasher
	GroupRepo                  idmports.GroupRepository
	AgentRepo                  idmports.AgentRepository
	MfaFactorRepo              authnports.MfaFactorRepository
	PasswordHistoryRepo        authnports.PasswordHistoryRepository
	PasswordResetTokenStore    authnports.PasswordResetTokenStore
	EmailChangeTokenStore      authnports.EmailChangeTokenStore
	EmailSender                authnports.EmailSender
	BreachedPasswordChecker    authnports.BreachedPasswordChecker
	LoginAttemptThrottle       authnports.LoginAttemptThrottle
	TrustedForwardedHops       int
	SentinelPasswordHash       string
	SessionManager             *authusecases.SessionManager
	AuthnResolver              authdomain.AuthenticationContextResolver
	WsFedRPRepo                wsfederationports.WsFedRelyingPartyRepository
	SamlSPRepo                 samlports.SamlServiceProviderRepository
	FederationSigner           *samltoken.Signer
	ApplicationRepo            appports.ApplicationRepository
	ApplicationAssignmentRepo  appports.AssignmentRepository
	Emit                       func(spec.DomainEvent)
	HealthInfo                 HealthInfo
}

// HealthInfo は bootstrap が決定した実行時構成のラベル。
// /health がそのまま JSON で返すだけの読み取り専用情報を保持する。
type HealthInfo struct {
	Persistence   string
	EventSink     string
	Observability string
	AuthZEN       string
}
