// Package http: identity management bounded context の HTTP アダプタ。
//
// ユーザー・グループ・エージェントの管理 API と、エンドユーザー自身の
// profile / email / data export の self-service API を所有する。
package http

import (
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

// Deps は support.Deps を埋め込む薄いラッパ。ハンドラを本 bounded context の
// メソッドとして保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*support.Deps
}

func RegisterRoutes(g *echo.Group, cd *support.Deps) {
	d := Deps{cd}
	g.GET("/api/account/summary", d.handleGetAccountSummary)
	g.GET("/api/account/profile", d.handleGetAccountProfile)
	g.PATCH("/api/account/profile", d.handleUpdateAccountProfile)
	g.POST("/api/account/email/change_request", d.handleRequestEmailChange)
	g.GET("/api/account/email/verify_context", d.handleEmailVerifyContext)
	g.POST("/api/account/email/verify", d.handleConfirmEmailChange)
	g.GET("/api/account/data_export", d.handleExportAccountData)
	g.GET("/api/admin/users", d.handleListAdminUsers)
	g.GET("/api/admin/users/:sub", d.handleGetAdminUser)
	g.POST("/api/admin/users", d.handleCreateAdminUser)
	g.PATCH("/api/admin/users/:sub", d.handleUpdateAdminUser)
	g.POST("/api/admin/users/:sub/disable", d.handleDisableAdminUser)
	g.POST("/api/admin/users/:sub/enable", d.handleEnableAdminUser)
	g.DELETE("/api/admin/users/:sub", d.handleDeleteAdminUser)
	g.POST("/api/admin/users/:sub/restore", d.handleRestoreAdminUser)
	g.POST("/api/admin/users/:sub/required_actions", d.handleSetUserRequiredAction)
	g.DELETE("/api/admin/users/:sub/required_actions/:action", d.handleClearUserRequiredAction)
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
}
