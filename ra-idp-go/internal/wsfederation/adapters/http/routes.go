// Package http は WsFederation bounded context の HTTP アダプタ (wi-61)。
//
// WS-Federation passive requestor profile のブラウザエンドポイントを所有する。
// 共有基盤 support.Deps を受け取り、shared/adapters/http/server から tenant 解決済みグループに登録される。
package http

import (
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

// Deps は support.Deps を埋め込む薄いラッパ。
type Deps struct {
	*support.Deps
}

// RegisterRoutes は WS-Federation passive のエンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *support.Deps) {
	d := Deps{cd}
	g.GET("/wsfed", d.handleWsFed)
	g.GET("/federationmetadata/2007-06/federationmetadata.xml", d.handleFederationMetadata)
	g.GET("/trust/mex", d.handleTrustMEX)
	g.POST("/trust/usernamemixed", d.handleWsTrustUsernameMixed)
	g.GET("/api/admin/wsfed/relying-parties", d.handleListRelyingParties)
	g.POST("/api/admin/wsfed/relying-parties", d.handleUpsertRelyingParty)
	g.DELETE("/api/admin/wsfed/relying-parties", d.handleDeleteRelyingParty)
	g.POST("/api/admin/wsfed/entra-federation", d.handleConfigureEntraFederation)
}
