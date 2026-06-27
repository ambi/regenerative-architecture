// Package http は Saml bounded context の HTTP アダプタ (wi-29)。
//
// SAML 2.0 Web Browser SSO Profile のブラウザエンドポイント (metadata / SSO / SLO) と、
// service provider 管理 API を所有する。共有基盤 core.Deps を受け取り、router (platform/http) から
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

// RegisterRoutes は SAML 2.0 IdP のエンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *core.Deps) {
	d := Deps{cd}
	g.GET("/saml/metadata", d.handleSamlMetadata)
	g.GET("/saml/sso", d.handleSamlSSORedirect)
	g.POST("/saml/sso", d.handleSamlSSOPost)
	g.GET("/saml/slo", d.handleSamlSLO)
	g.POST("/saml/slo", d.handleSamlSLO)
	g.GET("/api/admin/saml/service-providers", d.handleListServiceProviders)
	g.POST("/api/admin/saml/service-providers", d.handleUpsertServiceProvider)
	g.DELETE("/api/admin/saml/service-providers", d.handleDeleteServiceProvider)
}
