package core

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
)

const (
	CSRFCookie = "ra_idp_csrf"
	CSRFHeader = "X-CSRF-Token"
)

// VerifyBrowserRequest は Origin 一致と double-submit CSRF トークンを検証する。
// 認証必須のブラウザ向け POST/PATCH 系ハンドラが冒頭で呼ぶ。
func (d Deps) VerifyBrowserRequest(c *echo.Context) error {
	origin := c.Request().Header.Get("Origin")
	issuer, err := url.Parse(d.Issuer)
	if err != nil || origin == "" || origin != issuer.Scheme+"://"+issuer.Host {
		return WriteBrowserError(c, http.StatusForbidden, "invalid_origin", "Originが一致しません")
	}
	cookie, err := c.Cookie(CSRFCookie)
	header := c.Request().Header.Get(CSRFHeader)
	if err != nil || cookie.Value == "" || header == "" ||
		len(cookie.Value) != len(header) ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		return WriteBrowserError(c, http.StatusForbidden, "csrf_failed", "CSRF検証に失敗しました")
	}
	return nil
}

// EnsureCSRFCookie は CSRF cookie が無ければ発行し、そのトークン値を返す。
func (d Deps) EnsureCSRFCookie(c *echo.Context) (string, error) {
	if cookie, err := c.Cookie(CSRFCookie); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}
	value, err := randomToken(32)
	if err != nil {
		return "", err
	}
	c.SetCookie(&http.Cookie{ //nolint:gosec // Secure is enabled for HTTPS issuers; local HTTP development intentionally disables it.
		Name: CSRFCookie, Value: value, Path: TenantCookiePath(c),
		Secure: d.SecureCookies(), HttpOnly: false, SameSite: http.SameSiteStrictMode,
		MaxAge: 600,
	})
	return value, nil
}

// SecureCookies は issuer が HTTPS のときだけ cookie に Secure を付けるべきか返す。
func (d Deps) SecureCookies() bool {
	return strings.HasPrefix(d.Issuer, "https://")
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
