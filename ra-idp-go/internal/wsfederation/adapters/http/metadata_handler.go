package http

import (
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/wsfederation/adapters/metadata"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleFederationMetadata(c *echo.Context) error {
	endpoints := d.federationEndpoints(c)
	out, err := metadata.BuildFederationMetadata(support.RequestIssuer(c, d.Issuer), d.FederationSigner.Certificate(), endpoints, time.Now().UTC())
	if err != nil {
		return c.String(http.StatusInternalServerError, "federation metadata unavailable")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "application/xml; charset=utf-8", out)
}

func (d Deps) handleTrustMEX(c *echo.Context) error {
	out, err := metadata.BuildMEX(d.federationEndpoints(c))
	if err != nil {
		return c.String(http.StatusInternalServerError, "trust metadata unavailable")
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "application/xml; charset=utf-8", out)
}

func (d Deps) federationEndpoints(c *echo.Context) metadata.EndpointSet {
	base := strings.TrimRight(support.RequestIssuer(c, d.Issuer), "/")
	return metadata.EndpointSet{
		PassiveURL:        base + support.TenantRoute(c, "/wsfed"),
		ActiveURL:         base + support.TenantRoute(c, "/trust/usernamemixed"),
		MEXURL:            base + support.TenantRoute(c, "/trust/mex"),
		FederationMetaURL: base + support.TenantRoute(c, "/federationmetadata/2007-06/federationmetadata.xml"),
	}
}
