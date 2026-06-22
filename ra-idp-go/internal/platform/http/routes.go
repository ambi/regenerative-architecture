// Package http: Echo v5 を用いた HTTP アダプタの router。
// TS adapters/http/* に対応。
//
// 依存集約 (core.Deps) とテナント解決 middleware は core パッケージが持ち、
// 各エンドポイントのハンドラは責務ごとに *_handler.go へ分割している。
// このファイルではルーティング登録 (Register) のみを定義する。
package http

import (
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"
	tenancyhttp "ra-idp-go/internal/tenancy/adapters/http"

	"github.com/labstack/echo/v5"
)

// Deps は core.Deps を埋め込む薄いラッパ。ハンドラを所有コンテキストの
// メソッドとして保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*core.Deps
}

func Register(e *echo.Echo, cd core.Deps) {
	d := Deps{&cd}
	registerTenantRoutes(e.Group("", d.ResolveDefaultTenant), d)
	registerTenantRoutes(e.Group("/realms/:tenant_id", d.ResolvePathTenant), d)
	// テナント CRUD は default control-plane tenant のセッションでのみ操作するため
	// `/realms/default/admin/tenants` 配下に置き、cookie path と一致させる (ADR-032)。
	controlPlane := e.Group("/realms/"+spec.DefaultTenantID, d.ResolveControlPlaneTenant)
	tenancyhttp.RegisterControlPlaneRoutes(controlPlane, d.Deps)
	e.GET("/health", d.handleHealth)
}

func registerTenantRoutes(g *echo.Group, d Deps) {
	g.GET("/authorize", d.handleAuthorize)
	g.GET("/end_session", d.handleEndSession)
	g.POST("/end_session", d.handleEndSession)
	g.GET("/api/auth/transaction", d.handleTransaction)
	g.GET("/api/auth/account", d.handleAccountContext)
	g.GET("/api/account/summary", d.handleGetAccountSummary)
	g.GET("/api/account/profile", d.handleGetAccountProfile)
	g.PATCH("/api/account/profile", d.handleUpdateAccountProfile)
	g.POST("/api/account/email/change_request", d.handleRequestEmailChange)
	g.GET("/api/account/email/verify_context", d.handleEmailVerifyContext)
	g.POST("/api/account/email/verify", d.handleConfirmEmailChange)
	g.GET("/api/account/consents", d.handleListAccountConsents)
	g.POST("/api/account/consents/:client_id/revoke", d.handleRevokeAccountConsent)
	g.GET("/api/account/data_export", d.handleExportAccountData)
	g.POST("/api/account/step_up/start", d.handleStartStepUp)
	g.POST("/api/account/step_up/complete", d.handleCompleteStepUp)
	g.GET("/api/account/security", d.handleGetAccountSecurity)
	g.POST("/api/account/mfa/totp/enroll/start", d.handleStartTotpEnrollment)
	g.POST("/api/account/mfa/totp/enroll/confirm", d.handleConfirmTotpEnrollment)
	g.POST("/api/account/mfa/totp/remove", d.handleRemoveTotpFactor)
	g.GET("/api/account/signin_activity", d.handleListSignInActivity)
	g.GET("/api/account/sessions", d.handleListAccountSessions)
	g.POST("/api/account/sessions/:id/revoke", d.handleRevokeAccountSession)
	g.POST("/api/account/sessions/revoke_others", d.handleRevokeOtherAccountSessions)
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
	g.GET("/api/admin/users", d.handleListAdminUsers)
	g.GET("/api/admin/users/:sub", d.handleGetAdminUser)
	g.POST("/api/admin/users", d.handleCreateAdminUser)
	g.PATCH("/api/admin/users/:sub", d.handleUpdateAdminUser)
	g.POST("/api/admin/users/:sub/disable", d.handleDisableAdminUser)
	g.POST("/api/admin/users/:sub/enable", d.handleEnableAdminUser)
	g.DELETE("/api/admin/users/:sub", d.handleDeleteAdminUser)
	g.POST("/api/admin/users/:sub/required_actions", d.handleSetUserRequiredAction)
	g.DELETE("/api/admin/users/:sub/required_actions/:action", d.handleClearUserRequiredAction)
	g.GET("/api/admin/users/:sub/signin_activity", d.handleGetUserSignInActivity)
	g.GET("/api/admin/users/:sub/groups", d.handleListUserGroups)
	g.GET("/api/admin/groups", d.handleListGroups)
	g.GET("/api/admin/groups/:group_id", d.handleGetGroup)
	g.POST("/api/admin/groups", d.handleCreateGroup)
	g.PATCH("/api/admin/groups/:group_id", d.handleUpdateGroup)
	g.DELETE("/api/admin/groups/:group_id", d.handleDeleteGroup)
	g.POST("/api/admin/groups/:group_id/members/:user_sub", d.handleAddGroupMember)
	g.DELETE("/api/admin/groups/:group_id/members/:user_sub", d.handleRemoveGroupMember)
	g.GET("/api/admin/agents", d.handleListAgents)
	g.GET("/api/admin/agents/:agent_id", d.handleGetAgent)
	g.POST("/api/admin/agents", d.handleRegisterAgent)
	g.PATCH("/api/admin/agents/:agent_id", d.handleUpdateAgent)
	g.POST("/api/admin/agents/:agent_id/disable", d.handleDisableAgent)
	g.POST("/api/admin/agents/:agent_id/enable", d.handleEnableAgent)
	g.POST("/api/admin/agents/:agent_id/kill", d.handleKillAgent)
	g.DELETE("/api/admin/agents/:agent_id", d.handleDeleteAgent)
	g.POST("/api/admin/agents/:agent_id/credentials", d.handleBindAgentCredential)
	g.DELETE("/api/admin/agents/:agent_id/credentials/:client_id", d.handleUnbindAgentCredential)
	g.GET("/api/admin/clients", d.handleListAdminClients)
	g.GET("/api/admin/clients/:client_id", d.handleGetAdminClient)
	g.POST("/api/admin/clients", d.handleCreateAdminClient)
	g.PATCH("/api/admin/clients/:client_id", d.handleUpdateAdminClient)
	g.DELETE("/api/admin/clients/:client_id", d.handleDeleteAdminClient)
	g.GET("/api/admin/consents", d.handleListAdminConsents)
	g.GET("/api/admin/consents/:sub/:client_id", d.handleGetAdminConsent)
	g.DELETE("/api/admin/consents/:sub/:client_id", d.handleRevokeAdminConsent)
	g.GET("/api/admin/audit_events", d.handleListAdminAuditEvents)
	g.GET("/api/admin/audit_events/export", d.handleExportAdminAuditEvents)
	g.GET("/api/admin/audit_events/:id", d.handleGetAdminAuditEvent)
	g.GET("/api/admin/authentication_event_buckets", d.handleListAuthEventBuckets)
	g.GET("/api/admin/keys", d.handleListAdminKeys)
	g.GET("/api/admin/keys/:kid", d.handleGetAdminKey)
	g.POST("/api/admin/keys/rotate", d.handleRotateAdminKey)
	g.GET("/api/admin/policy/roles", d.handleListAdminRolePolicies)
	tenancyhttp.RegisterRoutes(g, d.Deps)
}
