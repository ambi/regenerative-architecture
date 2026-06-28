package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"ra-idp-go/internal/shared/spec"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantUserAttributeSchemaRepository は tenant ごとの custom 属性定義を保持する
// (ADR-040 / wi-19)。定義一覧は attributes JSONB 列に格納する。
type TenantUserAttributeSchemaRepository struct{ Pool *pgxpool.Pool }

func (r *TenantUserAttributeSchemaRepository) FindByTenant(
	ctx context.Context, tenantID string,
) (*spec.TenantUserAttributeSchema, error) {
	var (
		s          spec.TenantUserAttributeSchema
		attributes []byte
	)
	err := r.Pool.QueryRow(ctx,
		`SELECT tenant_id,attributes,updated_at FROM tenant_user_attribute_schemas WHERE tenant_id=$1`,
		tenantID,
	).Scan(&s.TenantID, &attributes, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(attributes) > 0 {
		if err := json.Unmarshal(attributes, &s.Attributes); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

func (r *TenantUserAttributeSchemaRepository) Save(ctx context.Context, s *spec.TenantUserAttributeSchema) error {
	attributes, err := json.Marshal(s.Attributes)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO tenant_user_attribute_schemas (tenant_id,attributes,updated_at)
VALUES ($1,$2,$3)
ON CONFLICT (tenant_id) DO UPDATE SET attributes=EXCLUDED.attributes,updated_at=EXCLUDED.updated_at`,
		s.TenantID, attributes, s.UpdatedAt)
	return err
}

func (r *TenantUserAttributeSchemaRepository) Delete(ctx context.Context, tenantID string) error {
	_, err := r.Pool.Exec(ctx, `DELETE FROM tenant_user_attribute_schemas WHERE tenant_id=$1`, tenantID)
	return err
}
