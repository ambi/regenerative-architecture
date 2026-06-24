// Package http は Federation bounded context の HTTP アダプタ (wi-61)。
//
// WS-Federation passive requestor profile のブラウザエンドポイントを所有する。
// 共有基盤 core.Deps を受け取り、router (platform/http) から tenant 解決済みグループに登録される。
package http

import (
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

// Deps は core.Deps を埋め込む薄いラッパ。
type Deps struct {
	*core.Deps
}

// RegisterRoutes は WS-Federation passive のエンドポイントを登録する。
func RegisterRoutes(g *echo.Group, cd *core.Deps) {
	d := Deps{cd}
	g.GET("/wsfed", d.handleWsFed)
	g.GET("/api/admin/wsfed/relying-parties", d.handleListRelyingParties)
	g.POST("/api/admin/wsfed/relying-parties", d.handleUpsertRelyingParty)
	g.DELETE("/api/admin/wsfed/relying-parties", d.handleDeleteRelyingParty)
}
