// Package http: Echo v5 を用いた HTTP アダプタの router。
// TS adapters/http/* に対応。
//
// 依存集約 (core.Deps) とテナント解決 middleware は core パッケージが持ち、
// 各エンドポイントのハンドラは責務ごとに *_handler.go へ分割している。
// このファイルではルーティング登録 (Register) のみを定義する。
package http

import (
	authhttp "ra-idp-go/internal/authentication/adapters/http"
	federationhttp "ra-idp-go/internal/federation/adapters/http"
	idmhttp "ra-idp-go/internal/identitymanagement/adapters/http"
	oauth2http "ra-idp-go/internal/oauth2/adapters/http"
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
	oauth2http.RegisterRoutes(g, d.Deps)
	authhttp.RegisterRoutes(g, d.Deps)
	idmhttp.RegisterRoutes(g, d.Deps)
	tenancyhttp.RegisterRoutes(g, d.Deps)
	federationhttp.RegisterRoutes(g, d.Deps)
}
