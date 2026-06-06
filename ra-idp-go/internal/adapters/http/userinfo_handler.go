// /userinfo
package http

import (
	"net/http"
	"strings"

	"ra-idp-go/internal/oauth2/usecases"

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
	res, err := usecases.UserInfo(c.Request().Context(), d.UserRepo, d.Authorizer, usecases.UserInfoInput{
		Scopes: strings.Fields(intro.Scope), Sub: intro.Sub, Active: intro.Active, ClientID: intro.ClientID,
	})
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
