// Package http: Echo v5 を用いた HTTP アダプタ。
// TS adapters/http/* に対応。
//
// 各エンドポイントは責務ごとに *_handler.go へ分割している。
// このファイルでは依存集約 (Deps) とルーティング登録 (Register) のみを定義する。
package http

import (
	"ra-idp-go/internal/adapters/crypto"
	authdomain "ra-idp-go/internal/authentication/domain"
	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"

	"github.com/labstack/echo/v5"
)

type Deps struct {
	Issuer                     string
	SCL                        *spec.SCL
	TenantRepo                 tenantports.TenantRepository
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
	Authorizer                 oauthports.Authorizer
	JWKResolver                *crypto.JWKResolver
	PasswordHasher             authports.PasswordHasher
	MfaFactorRepo              authports.MfaFactorRepository
	PasswordHistoryRepo        authports.PasswordHistoryRepository
	PasswordResetTokenStore    authports.PasswordResetTokenStore
	EmailSender                authports.EmailSender
	BreachedPasswordChecker    authports.BreachedPasswordChecker
	SessionManager             *authusecases.SessionManager
	AuthnResolver              authdomain.AuthenticationContextResolver
	Emit                       func(spec.DomainEvent)
	HealthInfo                 HealthInfo
}

func Register(e *echo.Echo, d Deps) {
	registerTenantRoutes(e.Group("", d.resolveDefaultTenant), d)
	registerTenantRoutes(e.Group("/realms/:tenant_id", d.resolvePathTenant), d)
	// テナント CRUD は default control-plane tenant のセッションでのみ操作するため
	// `/realms/default/admin/tenants` 配下に置き、cookie path と一致させる (ADR-032)。
	controlPlane := e.Group("/realms/"+spec.DefaultTenantID, d.resolveControlPlaneTenant)
	controlPlane.GET("/admin/tenants", d.handleListTenants)
	controlPlane.GET("/admin/tenants/:tenant_id", d.handleGetTenant)
	controlPlane.POST("/admin/tenants", d.handleCreateTenant)
	controlPlane.PATCH("/admin/tenants/:tenant_id", d.handleUpdateTenant)
	controlPlane.POST("/admin/tenants/:tenant_id/disable", d.handleDisableTenant)
	controlPlane.POST("/admin/tenants/:tenant_id/enable", d.handleEnableTenant)
	e.GET("/health", d.handleHealth)
}

func registerTenantRoutes(g *echo.Group, d Deps) {
	g.GET("/authorize", d.handleAuthorize)
	g.GET("/end_session", d.handleEndSession)
	g.POST("/end_session", d.handleEndSession)
	g.GET("/api/auth/transaction", d.handleTransaction)
	g.GET("/api/auth/account", d.handleAccountContext)
	g.POST("/api/auth/login", d.handleLoginAPI)
	g.POST("/api/auth/change_password", d.handleChangePasswordAPI)
	g.GET("/api/auth/password_reset_context", d.handlePasswordResetContext)
	g.POST("/api/auth/forgot_password", d.handleForgotPasswordAPI)
	g.POST("/api/auth/reset_password", d.handleResetPasswordAPI)
	g.POST("/api/auth/consent", d.handleConsentAPI)
	g.POST("/api/auth/totp", d.handleTOTPAPI)
	g.GET("/api/auth/device", d.handleDeviceContext)
	g.POST("/api/auth/device", d.handleDeviceAPI)
	g.POST("/token", d.handleToken)
	g.POST("/revoke", d.handleRevoke)
	g.POST("/introspect", d.handleIntrospect)
	g.GET("/userinfo", d.handleUserInfo)
	g.POST("/userinfo", d.handleUserInfo)
	g.POST("/register", d.handleRegisterClient)
	g.POST("/par", d.handlePAR)
	g.POST("/device_authorization", d.handleDeviceAuthorization)
	g.GET("/.well-known/openid-configuration", d.handleDiscovery)
	g.GET("/.well-known/oauth-authorization-server", d.handleDiscovery)
	g.GET("/jwks", d.handleJWKS)
	g.GET("/admin/users", d.handleListAdminUsers)
	g.GET("/admin/users/:sub", d.handleGetAdminUser)
	g.POST("/admin/users", d.handleCreateAdminUser)
	g.PATCH("/admin/users/:sub", d.handleUpdateAdminUser)
	g.POST("/admin/users/:sub/disable", d.handleDisableAdminUser)
	g.POST("/admin/users/:sub/enable", d.handleEnableAdminUser)
	g.GET("/admin/clients", d.handleListAdminClients)
	g.GET("/admin/clients/:client_id", d.handleGetAdminClient)
	g.POST("/admin/clients", d.handleCreateAdminClient)
	g.PATCH("/admin/clients/:client_id", d.handleUpdateAdminClient)
	g.DELETE("/admin/clients/:client_id", d.handleDeleteAdminClient)
	g.GET("/admin/consents", d.handleListAdminConsents)
	g.GET("/admin/consents/:sub/:client_id", d.handleGetAdminConsent)
	g.DELETE("/admin/consents/:sub/:client_id", d.handleRevokeAdminConsent)
	g.GET("/api/admin/users", d.handleListAdminUsers)
	g.GET("/api/admin/users/:sub", d.handleGetAdminUser)
	g.POST("/api/admin/users", d.handleCreateAdminUser)
	g.PATCH("/api/admin/users/:sub", d.handleUpdateAdminUser)
	g.POST("/api/admin/users/:sub/disable", d.handleDisableAdminUser)
	g.POST("/api/admin/users/:sub/enable", d.handleEnableAdminUser)
	g.GET("/api/admin/clients", d.handleListAdminClients)
	g.GET("/api/admin/clients/:client_id", d.handleGetAdminClient)
	g.POST("/api/admin/clients", d.handleCreateAdminClient)
	g.PATCH("/api/admin/clients/:client_id", d.handleUpdateAdminClient)
	g.DELETE("/api/admin/clients/:client_id", d.handleDeleteAdminClient)
	g.GET("/api/admin/consents", d.handleListAdminConsents)
	g.GET("/api/admin/consents/:sub/:client_id", d.handleGetAdminConsent)
	g.DELETE("/api/admin/consents/:sub/:client_id", d.handleRevokeAdminConsent)
}
