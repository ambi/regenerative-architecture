package http

import (
	"net/http"
	"strings"

	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"

	"github.com/labstack/echo/v5"
)

func (d Deps) resolveDefaultTenant(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return d.resolveTenant(c, next, spec.DefaultTenantID, true)
	}
}

func (d Deps) resolvePathTenant(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return d.resolveTenant(c, next, c.Param("tenant_id"), false)
	}
}

func (d Deps) resolveTenant(c *echo.Context, next echo.HandlerFunc, tenantID string, bare bool) error {
	if d.TenantRepo == nil {
		if tenantID != spec.DefaultTenantID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant_not_found"})
		}
		tenant := &spec.Tenant{ID: spec.DefaultTenantID, Status: spec.TenantStatusActive}
		issuer := tenantIssuer(d.Issuer, tenant.ID)
		if bare && d.LegacyBareIssuer {
			issuer = strings.TrimSuffix(d.Issuer, "/")
		}
		c.SetRequest(c.Request().WithContext(tenancy.WithTenant(c.Request().Context(), tenant, issuer)))
		return next(c)
	}
	tenant, err := d.TenantRepo.FindByID(c.Request().Context(), tenantID)
	if err != nil {
		return err
	}
	if tenant == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant_not_found"})
	}
	if tenant.Status != spec.TenantStatusActive || tenant.DisabledAt != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "tenant is unavailable"))
	}
	issuer := tenantIssuer(d.Issuer, tenant.ID)
	if bare && d.LegacyBareIssuer {
		issuer = strings.TrimSuffix(d.Issuer, "/")
	}
	c.SetRequest(c.Request().WithContext(tenancy.WithTenant(c.Request().Context(), tenant, issuer)))
	return next(c)
}

func tenantIssuer(base, tenantID string) string {
	return strings.TrimSuffix(base, "/") + "/realms/" + tenantID
}

func requestTenantID(c *echo.Context) string {
	return tenancy.TenantID(c.Request().Context())
}

func requestIssuer(c *echo.Context, fallback string) string {
	return tenancy.Issuer(c.Request().Context(), fallback)
}

func tenantRoute(c *echo.Context, path string) string {
	if tenantID := c.Param("tenant_id"); tenantID != "" {
		return "/realms/" + tenantID + path
	}
	return path
}

func tenantCookiePath(c *echo.Context) string {
	if tenantID := c.Param("tenant_id"); tenantID != "" {
		return "/realms/" + tenantID
	}
	return "/"
}
