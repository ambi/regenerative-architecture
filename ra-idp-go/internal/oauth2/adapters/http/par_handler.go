// /par (RFC 9126 Pushed Authorization Request)
package http

import (
	"net/http"
	"time"

	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/oauth2/usecases"

	"github.com/labstack/echo/v5"
)

func (d Deps) handlePAR(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, core.OAuthErrorBody("invalid_request", "form parse"))
	}
	clientStub, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	params := map[string]string{}
	for k, v := range c.Request().PostForm {
		if k == "client_id" || k == "client_secret" ||
			k == "client_assertion" || k == "client_assertion_type" {
			continue
		}
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	res, err := usecases.PushAuthorizationRequest(c.Request().Context(), usecases.PARDeps{
		ClientRepo: d.ClientRepo, Store: d.PARStore, AuthzDetailTypeRepo: d.AuthzDetailTypeRepo, Emit: d.Emit,
	}, usecases.PARInput{ClientID: clientStub.ID, Parameters: params}, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"request_uri": res.RequestURI, "expires_in": res.ExpiresIn,
	})
}
