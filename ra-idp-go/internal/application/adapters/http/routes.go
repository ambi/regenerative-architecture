// Package http は Application bounded context の HTTP アダプタ (wi-69)。
//
// 運用者向け Application カタログ (CRUD・protocol binding・割当) と、利用者ポータル向けの
// 割当済みアプリ一覧を所有する。共有基盤 support.Deps を受け取り、shared/adapters/http/server から
// tenant 解決済みグループに登録される。
package http

import (
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

// Deps は support.Deps を埋め込む薄いラッパ。
type Deps struct {
	*support.Deps
}

// RegisterRoutes は Application カタログの admin / account エンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *support.Deps) {
	d := Deps{cd}
	g.GET("/api/admin/applications", d.handleListApplications)
	g.POST("/api/admin/applications", d.handleCreateApplication)
	g.GET("/api/admin/applications/:application_id", d.handleGetApplication)
	g.PATCH("/api/admin/applications/:application_id", d.handleUpdateApplication)
	g.DELETE("/api/admin/applications/:application_id", d.handleDeleteApplication)
	g.POST("/api/admin/applications/:application_id/icon", d.handleUploadApplicationIcon)
	g.DELETE("/api/admin/applications/:application_id/icon", d.handleDeleteApplicationIcon)
	g.POST("/api/admin/applications/:application_id/bindings", d.handleAttachBinding)
	g.DELETE("/api/admin/applications/:application_id/bindings/:binding_type", d.handleDetachBinding)
	g.PATCH("/api/admin/applications/:application_id/oidc", d.handleUpdateOIDCConfig)
	g.PATCH("/api/admin/applications/:application_id/wsfed", d.handleUpdateWsFedConfig)
	g.PATCH("/api/admin/applications/:application_id/saml", d.handleUpdateSamlConfig)
	g.GET("/api/admin/applications/:application_id/assignments", d.handleListAssignments)
	g.POST("/api/admin/applications/:application_id/assignments", d.handleAssignApplication)
	g.DELETE("/api/admin/applications/:application_id/assignments/:subject_type/:subject_id", d.handleUnassignApplication)
	g.PUT("/api/admin/applications/:application_id/categories", d.handleSetApplicationCategories)
	g.GET("/api/admin/application-categories", d.handleListCategories)
	g.POST("/api/admin/application-categories", d.handleCreateCategory)
	g.PATCH("/api/admin/application-categories/:category_id", d.handleUpdateCategory)
	g.DELETE("/api/admin/application-categories/:category_id", d.handleDeleteCategory)
	g.GET("/api/account/applications", d.handleListMyApplications)
	g.GET("/api/account/applications/order", d.handleGetMyApplicationOrder)
	g.PUT("/api/account/applications/order", d.handleReorderMyApplications)
	g.GET("/application-icons/:application_id/:object_key", d.handleGetApplicationIcon)
}
