// Package server: Echo v5 を用いた HTTP アダプタの router。
//
// 依存集約 (support.Deps) とテナント解決 middleware は support パッケージが持ち、
// 各エンドポイントのハンドラは責務ごとに *_handler.go へ分割している。
// このファイルではルーティング登録 (Register) のみを定義する。
package server

import (
	apphttp "ra-idp-go/internal/application/adapters/http"
	authhttp "ra-idp-go/internal/authentication/adapters/http"
	idmhttp "ra-idp-go/internal/identitymanagement/adapters/http"
	oauth2http "ra-idp-go/internal/oauth2/adapters/http"
	samlhttp "ra-idp-go/internal/saml/adapters/http"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"
	tenancyhttp "ra-idp-go/internal/tenancy/adapters/http"
	wsfederationhttp "ra-idp-go/internal/wsfederation/adapters/http"

	"github.com/labstack/echo/v5"
)

// Deps は support.Deps を埋め込む薄いラッパ。ハンドラを所有コンテキストの
// メソッドとして保持するためのキャリアで、固有のフィールドは持たない。
type Deps struct {
	*support.Deps
}

func Register(e *echo.Echo, cd support.Deps) {
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
	wsfederationhttp.RegisterRoutes(g, d.Deps)
	samlhttp.RegisterRoutes(g, d.Deps)
	apphttp.RegisterRoutes(g, d.Deps)
}
