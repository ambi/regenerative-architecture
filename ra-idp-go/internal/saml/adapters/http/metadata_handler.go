package http

import (
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/saml/adapters/metadata"
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

// handleSamlMetadata は realm 単位の SAML 2.0 IdP metadata を公開する。
func (d Deps) handleSamlMetadata(c *echo.Context) error {
	if d.FederationSigner == nil {
		return c.String(http.StatusInternalServerError, "saml metadata unavailable")
	}
	base := strings.TrimRight(support.RequestIssuer(c, d.Issuer), "/")
	endpoints := metadata.Endpoints{
		SSOURL: base + support.TenantRoute(c, "/saml/sso"),
		SLOURL: base + support.TenantRoute(c, "/saml/slo"),
	}
	out, err := metadata.BuildIDPMetadata(support.RequestIssuer(c, d.Issuer), d.FederationSigner.Certificate(), endpoints, time.Now().UTC())
	if err != nil {
		return c.String(http.StatusInternalServerError, "saml metadata unavailable")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "application/xml; charset=utf-8", out)
}
