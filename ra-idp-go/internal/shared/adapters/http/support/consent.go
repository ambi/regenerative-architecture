package support

import (
	"errors"
	"net/http"

	oauthusecases "ra-idp-go/internal/oauth2/usecases"

	"github.com/labstack/echo/v5"
)

// ConsentDeps は consent ユースケースの依存を束ねる。管理系の consent 操作と
// アカウント側の同意一覧・データエクスポートが共有する。
func (d Deps) ConsentDeps() oauthusecases.ConsentDeps {
	return oauthusecases.ConsentDeps{ConsentRepo: d.ConsentRepo, Emit: d.Emit}
}

// WriteConsentError は consent 操作のドメインエラーを HTTP エラーへ変換する。
func (d Deps) WriteConsentError(c *echo.Context, err error) error {
	if errors.Is(err, oauthusecases.ErrConsentNotFound) {
		return WriteBrowserError(c, http.StatusNotFound, "consent_not_found", "同意記録が存在しません")
	}
	return err
}
