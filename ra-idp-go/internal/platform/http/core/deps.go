// Package core: HTTP アダプタの共有基盤。
//
// 複数コンテキスト (tenancy / authentication / oauth2) の adapters/http が
// 共通で使う依存集約 (Deps)・テナント解決 middleware・横断ヘルパのみを置く。
// 各コンテキストの adapters/http は core を import し、router (platform/http) が
// 各コンテキストの RegisterRoutes を集約する。core ← context ← router の一方向。
package core

import (
	authdomain "ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	idmports "ra-idp-go/internal/identitymanagement/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/platform/crypto"
	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
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
	AuthEventBucketStore       authports.AuthEventBucketStore
	Authorizer                 oauthports.Authorizer
	JWKResolver                *crypto.JWKResolver
	PasswordHasher             authports.PasswordHasher
	GroupRepo                  idmports.GroupRepository
	AgentRepo                  idmports.AgentRepository
	MfaFactorRepo              authports.MfaFactorRepository
	PasswordHistoryRepo        authports.PasswordHistoryRepository
	PasswordResetTokenStore    authports.PasswordResetTokenStore
	EmailChangeTokenStore      authports.EmailChangeTokenStore
	EmailSender                authports.EmailSender
	BreachedPasswordChecker    authports.BreachedPasswordChecker
	LoginAttemptThrottle       authports.LoginAttemptThrottle
	TrustedForwardedHops       int
	SentinelPasswordHash       string
	SessionManager             *authusecases.SessionManager
	AuthnResolver              authdomain.AuthenticationContextResolver
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
