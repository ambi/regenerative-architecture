// /userinfo
package http

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleUserInfo(c *echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "Bearer token が必要"))
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	intro, err := d.TokenIntrospector.IntrospectAccessToken(c.Request().Context(), token)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if !intro.Active {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "トークンが無効"))
	}
	if d.AccessTokenDenylist != nil && intro.JTI != "" {
		revoked, err := d.AccessTokenDenylist.IsRevoked(c.Request().Context(), intro.JTI)
		if err != nil {
			return writeOAuthError(c, err)
		}
		if revoked {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "トークンが失効済みです"))
		}
	}
	if intro.SenderConstraint != nil && intro.SenderConstraint.Type == spec.SenderConstraintMTLS {
		cert, err := crypto.ParseClientCertificateHeader(c.Request().Header.Get(clientCertHeader))
		if err != nil || subtle.ConstantTimeCompare(
			[]byte(cert.ThumbprintS256),
			[]byte(intro.SenderConstraint.X5TS256),
		) != 1 {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "mTLS 証明書バインドが一致しません"))
		}
	}
	res, err := usecases.UserInfo(c.Request().Context(), d.UserRepo, d.Authorizer, usecases.UserInfoInput{
		Scopes: strings.Fields(intro.Scope), Sub: intro.Sub, Active: intro.Active, ClientID: intro.ClientID,
	})
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
