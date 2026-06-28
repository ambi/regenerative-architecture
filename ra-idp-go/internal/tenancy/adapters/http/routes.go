// Package http: tenancy コンテキストの HTTP アダプタ。
//
// テナント設定・ユーザ属性スキーマ・テナント CRUD (control-plane) のハンドラを所有し、
// 共有基盤 support.Deps を受け取って shared/adapters/http/server から登録される。
package http

import (
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

// Deps は support.Deps を埋め込む薄いラッパ。ハンドラを本コンテキストのメソッドとして
// 保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*support.Deps
}

// RegisterRoutes はテナント解決済みグループに、テナント単位の admin 設定・
// ユーザ属性スキーマのエンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *support.Deps) {
	d := Deps{cd}
	g.GET("/api/admin/settings", d.handleGetAdminSettings)
	g.PATCH("/api/admin/settings", d.handleUpdateAdminSettings)
	g.GET("/api/admin/tenant/user_attribute_schema", d.handleGetUserAttributeSchema)
	g.PUT("/api/admin/tenant/user_attribute_schema", d.handleUpdateUserAttributeSchema)
}

// RegisterControlPlaneRoutes はテナント CRUD を control-plane グループ
// (/realms/default 配下、ADR-032) に登録する。
func RegisterControlPlaneRoutes(g *echo.Group, cd *support.Deps) {
	d := Deps{cd}
	g.GET("/admin/tenants", d.handleListTenants)
	g.GET("/admin/tenants/:tenant_id", d.handleGetTenant)
	g.POST("/admin/tenants", d.handleCreateTenant)
	g.PATCH("/admin/tenants/:tenant_id", d.handleUpdateTenant)
	g.POST("/admin/tenants/:tenant_id/disable", d.handleDisableTenant)
	g.POST("/admin/tenants/:tenant_id/enable", d.handleEnableTenant)
}
