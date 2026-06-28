package usecases

import (
	"context"
	"errors"
	"time"

	"ra-idp-go/internal/shared/spec"
	tenantports "ra-idp-go/internal/tenancy/ports"
)

// ErrInvalidUserAttributeSchema は custom 属性定義が不正 (key 衝突 / 重複 / 形式違反)
// のときに返す (ADR-040)。
var ErrInvalidUserAttributeSchema = errors.New("invalid attribute schema")

// GetUserAttributeSchema は tenant の custom 属性定義を返す。未定義のテナントには
// 空集合の schema を返し、呼び出し側が常に non-nil を扱えるようにする。
func GetUserAttributeSchema(
	ctx context.Context, repo tenantports.TenantUserAttributeSchemaRepository, tenantID string,
) (*spec.TenantUserAttributeSchema, error) {
	schema, err := repo.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return &spec.TenantUserAttributeSchema{TenantID: tenantID, Attributes: []spec.UserAttributeDef{}}, nil
	}
	return schema, nil
}

// UpdateUserAttributeSchema は tenant の custom 属性定義を全置換する。各定義を検証し、
// 組み込み属性との key 衝突・重複 key を拒否したうえで保存する (ADR-040)。
func UpdateUserAttributeSchema(
	ctx context.Context, repo tenantports.TenantUserAttributeSchemaRepository,
	tenantID string, defs []spec.UserAttributeDef, now time.Time,
) (*spec.TenantUserAttributeSchema, error) {
	if defs == nil {
		defs = []spec.UserAttributeDef{}
	}
	schema := &spec.TenantUserAttributeSchema{
		TenantID:   tenantID,
		Attributes: defs,
		UpdatedAt:  now,
	}
	if err := schema.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidUserAttributeSchema, err)
	}
	if err := repo.Save(ctx, schema); err != nil {
		return nil, err
	}
	return schema, nil
}
