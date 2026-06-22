// Package http: authentication コンテキストの HTTP アダプタ。
//
// アカウント自己管理 (profile/consent/session/security/mfa/data export/step-up)、
// パスワード変更・リセット・メール変更、ユーザ/グループ/エージェントの管理 API、
// 認証イベントバケットの閲覧を所有する。共有基盤 core.Deps を受け取り router から
// 登録される。
package http

import (
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

// Deps は core.Deps を埋め込む薄いラッパ。ハンドラを本コンテキストのメソッドとして
// 保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*core.Deps
}

// RegisterRoutes はテナント解決済みグループに authentication コンテキストの
// エンドポイントを登録する。パス・メソッド・middleware は分割前と一致する。
func RegisterRoutes(g *echo.Group, cd *core.Deps) {
	d := Deps{cd}
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
	g.POST("/api/auth/change_password", d.handleChangePasswordAPI)
	g.GET("/api/auth/password_reset_context", d.handlePasswordResetContext)
	g.POST("/api/auth/forgot_password", d.handleForgotPasswordAPI)
	g.POST("/api/auth/reset_password", d.handleResetPasswordAPI)
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
	g.GET("/api/admin/authentication_event_buckets", d.handleListAuthEventBuckets)
}
