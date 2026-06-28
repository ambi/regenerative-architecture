// Package http: oauth2 コンテキストの HTTP アダプタ。
//
// OAuth 2.0 / OIDC のプロトコルエンドポイント (authorize/token/introspect/revoke/
// userinfo/par/device/discovery/register) と、認可トランザクションのフロントエンドである
// 対話ログイン (login/totp/consent/end_session)、および client/consent/key/audit_event/
// role_policy の管理 API を所有する。共有基盤 core.Deps を受け取り router から登録される。
package http

import (
	"ra-idp-go/internal/infrastructure/http/core"

	"github.com/labstack/echo/v5"
)

// Deps は core.Deps を埋め込む薄いラッパ。ハンドラを本コンテキストのメソッドとして
// 保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*core.Deps
}

// RegisterRoutes はテナント解決済みグループに oauth2 コンテキストのエンドポイントを
// 登録する。パス・メソッド・middleware は分割前と一致する。
func RegisterRoutes(g *echo.Group, cd *core.Deps) {
	d := Deps{cd}
	g.GET("/authorize", d.handleAuthorize)
	g.GET("/end_session", d.handleEndSession)
	g.POST("/end_session", d.handleEndSession)
	g.GET("/api/auth/transaction", d.handleTransaction)
	g.POST("/api/auth/login", d.handleLoginAPI)
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
	g.GET("/api/admin/clients", d.handleListAdminOAuth2Clients)
	g.GET("/api/admin/clients/:client_id", d.handleGetAdminOAuth2Client)
	g.POST("/api/admin/clients", d.handleCreateAdminOAuth2Client)
	g.PATCH("/api/admin/clients/:client_id", d.handleUpdateAdminOAuth2Client)
	g.DELETE("/api/admin/clients/:client_id", d.handleDeleteAdminOAuth2Client)
	g.GET("/api/admin/authorization-detail-types", d.handleListAuthorizationDetailTypes)
	g.GET("/api/admin/authorization-detail-types/:type", d.handleGetAuthorizationDetailType)
	g.POST("/api/admin/authorization-detail-types", d.handleCreateAuthorizationDetailType)
	g.PATCH("/api/admin/authorization-detail-types/:type", d.handleUpdateAuthorizationDetailType)
	g.DELETE("/api/admin/authorization-detail-types/:type", d.handleDeleteAuthorizationDetailType)
	g.GET("/api/admin/consents", d.handleListAdminConsents)
	g.GET("/api/admin/consents/:sub/:client_id", d.handleGetAdminConsent)
	g.DELETE("/api/admin/consents/:sub/:client_id", d.handleRevokeAdminConsent)
	g.GET("/api/admin/audit_events", d.handleListAdminAuditEvents)
	g.GET("/api/admin/audit_events/export", d.handleExportAdminAuditEvents)
	g.GET("/api/admin/audit_events/:id", d.handleGetAdminAuditEvent)
	g.GET("/api/admin/keys", d.handleListAdminKeys)
	g.GET("/api/admin/keys/:kid", d.handleGetAdminKey)
	g.POST("/api/admin/keys/rotate", d.handleRotateAdminKey)
	g.GET("/api/admin/policy/roles", d.handleListAdminRolePolicies)
}
