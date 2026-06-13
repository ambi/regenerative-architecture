package tenancy

import (
	"context"
	"strings"

	"ra-idp-go/internal/spec"
)

type contextKey string

const (
	tenantKey    contextKey = "tenant"
	issuerKey    contextKey = "tenant-issuer"
	urlPrefixKey contextKey = "tenant-url-prefix"
)

func WithTenant(ctx context.Context, tenant *spec.Tenant, issuer, urlPrefix string) context.Context {
	ctx = context.WithValue(ctx, tenantKey, tenant)
	ctx = context.WithValue(ctx, issuerKey, strings.TrimSuffix(issuer, "/"))
	return context.WithValue(ctx, urlPrefixKey, strings.TrimSuffix(urlPrefix, "/"))
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

// URLPrefix は middleware が解決した URL prefix (`/realms/{id}` または空文字) を返す。
// cookie path や redirect URL の組み立てに使う。
func URLPrefix(ctx context.Context) string {
	prefix, _ := ctx.Value(urlPrefixKey).(string)
	return prefix
}
