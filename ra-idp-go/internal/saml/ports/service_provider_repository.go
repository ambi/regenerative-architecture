// Package ports は Saml bounded context の永続境界 (port) を定義する (wi-29)。
package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

// SamlServiceProviderRepository は SAML 2.0 service provider 登録の永続境界 (wi-29)。
// entityID で識別し、許可 ACS・audience・NameID format・claim policy を保持する。
type SamlServiceProviderRepository interface {
	// FindByEntityID は entityID に一致する SP を返す。存在しなければ (nil, nil)。
	FindByEntityID(ctx context.Context, tenantID, entityID string) (*spec.SamlServiceProvider, error)
	// ListByTenant はテナント内の SP を entityID 昇順で返す。
	ListByTenant(ctx context.Context, tenantID string) ([]*spec.SamlServiceProvider, error)
	// Save は SP を upsert する。
	Save(ctx context.Context, sp *spec.SamlServiceProvider) error
	// Delete は entityID に一致する SP を削除する (冪等)。
	Delete(ctx context.Context, tenantID, entityID string) error
}
