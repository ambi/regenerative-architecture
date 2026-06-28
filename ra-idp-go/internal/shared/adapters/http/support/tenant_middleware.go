package support

import (
	"net/http"
	"strings"

	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"

	"github.com/labstack/echo/v5"
)

func (d Deps) ResolveDefaultTenant(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return d.resolveTenant(c, next, spec.DefaultTenantID, "", true)
	}
}

func (d Deps) ResolvePathTenant(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		tenantID := c.Param("tenant_id")
		return d.resolveTenant(c, next, tenantID, "/realms/"+tenantID, false)
	}
}

// ResolveControlPlaneTenant は固定で default テナントを resolve し、URL prefix
// /realms/default を ctx に載せる (cookie path 整合のため)。/realms/default/admin/tenants
// 等の control-plane 経路で使う。
func (d Deps) ResolveControlPlaneTenant(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		return d.resolveTenant(c, next, spec.DefaultTenantID, "/realms/"+spec.DefaultTenantID, false)
	}
}

func (d Deps) resolveTenant(c *echo.Context, next echo.HandlerFunc, tenantID, urlPrefix string, bare bool) error {
	if d.TenantRepo == nil {
		if tenantID != spec.DefaultTenantID {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant_not_found"})
		}
		tenant := &spec.Tenant{ID: spec.DefaultTenantID, Status: spec.TenantStatusActive}
		issuer := tenantIssuer(d.Issuer, tenant.ID)
		if bare && d.LegacyBareIssuer {
			issuer = strings.TrimSuffix(d.Issuer, "/")
		}
		c.SetRequest(c.Request().WithContext(tenancy.WithTenant(c.Request().Context(), tenant, issuer, urlPrefix)))
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
		return c.JSON(http.StatusBadRequest, OAuthErrorBody("invalid_request", "tenant is unavailable"))
	}
	issuer := tenantIssuer(d.Issuer, tenant.ID)
	if bare && d.LegacyBareIssuer {
		issuer = strings.TrimSuffix(d.Issuer, "/")
	}
	c.SetRequest(c.Request().WithContext(tenancy.WithTenant(c.Request().Context(), tenant, issuer, urlPrefix)))
	return next(c)
}

func tenantIssuer(base, tenantID string) string {
	return strings.TrimSuffix(base, "/") + "/realms/" + tenantID
}

func RequestTenantID(c *echo.Context) string {
	return tenancy.TenantID(c.Request().Context())
}

func RequestIssuer(c *echo.Context, fallback string) string {
	return tenancy.Issuer(c.Request().Context(), fallback)
}

// RequestHTU は DPoP proof の htu (RFC 9449 §4.2) として用いる、
// クエリ・フラグメント無しの絶対 URL を返す。
// テナント prefix `/realms/{id}` を含むパスでもクライアントが送ったままに復元する。
func RequestHTU(c *echo.Context, base string) string {
	return strings.TrimRight(base, "/") + c.Request().URL.Path
}

func TenantRoute(c *echo.Context, path string) string {
	if prefix := tenancy.URLPrefix(c.Request().Context()); prefix != "" {
		return prefix + path
	}
	return path
}

func TenantCookiePath(c *echo.Context) string {
	if prefix := tenancy.URLPrefix(c.Request().Context()); prefix != "" {
		return prefix
	}
	return "/"
}
