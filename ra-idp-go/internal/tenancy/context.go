package tenancy

import (
	"context"
	"strings"

	"ra-idp-go/internal/spec"
)

type contextKey string

const (
	tenantKey contextKey = "tenant"
	issuerKey contextKey = "tenant-issuer"
)

func WithTenant(ctx context.Context, tenant *spec.Tenant, issuer string) context.Context {
	ctx = context.WithValue(ctx, tenantKey, tenant)
	return context.WithValue(ctx, issuerKey, strings.TrimSuffix(issuer, "/"))
}

func Tenant(ctx context.Context) *spec.Tenant {
	tenant, _ := ctx.Value(tenantKey).(*spec.Tenant)
	return tenant
}

func TenantID(ctx context.Context) string {
	if tenant := Tenant(ctx); tenant != nil && tenant.ID != "" {
		return tenant.ID
	}
	return spec.DefaultTenantID
}

func Issuer(ctx context.Context, fallback string) string {
	issuer, _ := ctx.Value(issuerKey).(string)
	if issuer != "" {
		return issuer
	}
	return strings.TrimSuffix(fallback, "/")
}
