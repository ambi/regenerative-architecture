// /.well-known/openid-configuration + /jwks
package http

import (
	"net/http"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

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
	// RFC 9396 — テナントで Enabled な authorization_details type を広告する (ADR-050)。
	if d.AuthzDetailTypeRepo != nil {
		types, err := d.AuthzDetailTypeRepo.ListByTenant(c.Request().Context(), core.RequestTenantID(c))
		if err != nil {
			return writeOAuthError(c, err)
		}
		supported := make([]string, 0, len(types))
		for _, t := range types {
			if t.State == spec.DetailTypeEnabled {
				supported = append(supported, t.Type)
			}
		}
		if len(supported) > 0 {
			doc["authorization_details_types_supported"] = supported
		}
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
