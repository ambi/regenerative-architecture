package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

// TenantAttributeSchemaRepository は tenant ごとの custom attribute 定義集合
// (ADR-040) を保持する。tenant aggregate には埋め込まず独立 aggregate として
// 持ち、tenant 削除時に Delete で cascade する。
type TenantAttributeSchemaRepository interface {
	// FindByTenant は tenant の schema を返す。未定義なら nil, nil。
	FindByTenant(ctx context.Context, tenantID string) (*spec.TenantAttributeSchema, error)
	Save(ctx context.Context, schema *spec.TenantAttributeSchema) error
	Delete(ctx context.Context, tenantID string) error
}
