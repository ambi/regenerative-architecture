package http

import (
	"net/http"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// handleSamlSLO は SAML Single Logout を処理する。ローカルセッションを破棄し、要求元 SP の
// SingleLogoutService が登録されていれば許可済みの返送先へリダイレクトする。判定不能・未登録の
// 返送先へはリダイレクトしない (open redirect 防止)。
func (d Deps) handleSamlSLO(c *echo.Context) error {
	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)
	entityID := samlParam(c, "entityID", "sp")
	relayState := samlParam(c, "RelayState", "")

	if d.SessionManager != nil {
		_ = d.SessionManager.Revoke(ctx, c.Request().Header.Get("Cookie"))
	}
	d.clearSessionCookie(c)
	d.emit(&spec.SamlLogout{At: time.Now().UTC(), TenantID: tenantID, EntityID: entityID})

	if target := d.resolveLogoutRedirect(c, tenantID, entityID, relayState); target != "" {
		return c.Redirect(http.StatusSeeOther, target)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.String(http.StatusOK, "signed out")
}

// resolveLogoutRedirect は entityID で SP を解決し、登録済み SingleLogoutService URL を返す。
// SP / SLO URL が無ければ空文字を返し、リダイレクトしない。
func (d Deps) resolveLogoutRedirect(c *echo.Context, tenantID, entityID, relayState string) string {
	if entityID == "" || d.SamlSPRepo == nil {
		return ""
	}
	sp, err := d.SamlSPRepo.FindByEntityID(c.Request().Context(), tenantID, entityID)
	if err != nil || sp == nil || sp.SLOURL == "" {
		return ""
	}
	target := sp.SLOURL
	if relayState != "" {
		target += "?RelayState=" + relayState
	}
	return target
}

func (d Deps) clearSessionCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure は HTTPS issuer で有効化、ローカル HTTP 開発では意図的に無効。
		Name: authusecases.SessionCookie, Path: core.TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: -1,
	})
}

// samlParam はクエリ / フォームから primary を取り、空なら fallback キーを引く。
func samlParam(c *echo.Context, primary, fallback string) string {
	if v := c.QueryParam(primary); v != "" {
		return v
	}
	if v := c.FormValue(primary); v != "" {
		return v
	}
	if fallback != "" {
		if v := c.QueryParam(fallback); v != "" {
			return v
		}
	}
	return ""
}
