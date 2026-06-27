// Package http は Application bounded context の HTTP アダプタ (wi-69)。
//
// 運用者向け Application カタログ (CRUD・protocol binding・割当) と、利用者ポータル向けの
// 割当済みアプリ一覧を所有する。共有基盤 core.Deps を受け取り、router (platform/http) から
// tenant 解決済みグループに登録される。
package http

import (
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

// Deps は core.Deps を埋め込む薄いラッパ。
type Deps struct {
	*core.Deps
}

// RegisterRoutes は Application カタログの admin / account エンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *core.Deps) {
	d := Deps{cd}
	g.GET("/api/admin/applications", d.handleListApplications)
	g.POST("/api/admin/applications", d.handleCreateApplication)
	g.GET("/api/admin/applications/:application_id", d.handleGetApplication)
	g.PATCH("/api/admin/applications/:application_id", d.handleUpdateApplication)
	g.DELETE("/api/admin/applications/:application_id", d.handleDeleteApplication)
	g.POST("/api/admin/applications/:application_id/bindings", d.handleAttachBinding)
	g.DELETE("/api/admin/applications/:application_id/bindings/:binding_type", d.handleDetachBinding)
	g.GET("/api/admin/applications/:application_id/assignments", d.handleListAssignments)
	g.POST("/api/admin/applications/:application_id/assignments", d.handleAssignApplication)
	g.DELETE("/api/admin/applications/:application_id/assignments/:subject_type/:subject_id", d.handleUnassignApplication)
	g.GET("/api/account/applications", d.handleListMyApplications)
}
