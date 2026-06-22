// /.well-known/openid-configuration + /jwks
package http

import (
	"net/http"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleDiscovery(c *echo.Context) error {
	if d.SCL == nil {
		return writeOAuthError(c, usecases.NewOAuthError("server_error", "SCL not loaded"))
	}
	doc, err := d.SCL.BuildDiscoveryDocument(core.RequestIssuer(c, d.Issuer))
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, doc)
}

func (d Deps) handleJWKS(c *echo.Context) error {
	keys, err := d.KeyStore.GetAllKeys(c.Request().Context())
	if err != nil {
		return writeOAuthError(c, err)
	}
	jwks := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		jwks = append(jwks, k.PublicJWK)
	}
	return c.JSON(http.StatusOK, map[string]any{"keys": jwks})
}
