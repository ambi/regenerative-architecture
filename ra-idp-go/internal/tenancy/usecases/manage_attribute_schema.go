package usecases

import (
	"context"
	"errors"
	"time"

	"ra-idp-go/internal/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

// ErrInvalidAttributeSchema は custom 属性定義が不正 (key 衝突 / 重複 / 形式違反)
// のときに返す (ADR-040)。
var ErrInvalidAttributeSchema = errors.New("invalid attribute schema")

// GetAttributeSchema は tenant の custom 属性定義を返す。未定義のテナントには
// 空集合の schema を返し、呼び出し側が常に non-nil を扱えるようにする。
func GetAttributeSchema(
	ctx context.Context, repo tenantports.TenantAttributeSchemaRepository, tenantID string,
) (*spec.TenantAttributeSchema, error) {
	schema, err := repo.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return &spec.TenantAttributeSchema{TenantID: tenantID, Attributes: []spec.AttributeDef{}}, nil
	}
	return schema, nil
}

// UpdateAttributeSchema は tenant の custom 属性定義を全置換する。各定義を検証し、
// 組み込み属性との key 衝突・重複 key を拒否したうえで保存する (ADR-040)。
func UpdateAttributeSchema(
	ctx context.Context, repo tenantports.TenantAttributeSchemaRepository,
	tenantID string, defs []spec.AttributeDef, now time.Time,
) (*spec.TenantAttributeSchema, error) {
	if defs == nil {
		defs = []spec.AttributeDef{}
	}
	schema := &spec.TenantAttributeSchema{
		TenantID:   tenantID,
		Attributes: defs,
		UpdatedAt:  now,
	}
	if err := schema.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidAttributeSchema, err)
	}
	if err := repo.Save(ctx, schema); err != nil {
		return nil, err
	}
	return schema, nil
}
