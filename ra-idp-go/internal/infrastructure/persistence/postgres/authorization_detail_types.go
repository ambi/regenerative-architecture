package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// AuthorizationDetailTypeRepository は RFC 9396 authorization_details の type 定義
// (ADR-050) を PostgreSQL に永続化する。schema は JSONB として保持する。すべての
// 参照はテナント境界に閉じる。
type AuthorizationDetailTypeRepository struct{ Pool *pgxpool.Pool }

const authorizationDetailTypeSelect = `SELECT tenant_id,type,description,schema,display_template,
state,created_at,updated_at FROM authorization_detail_types`

func scanAuthorizationDetailType(row rowScanner) (*spec.AuthorizationDetailType, error) {
	var t spec.AuthorizationDetailType
	var schema []byte
	err := row.Scan(&t.TenantID, &t.Type, &t.Description, &schema, &t.DisplayTemplate,
		&t.State, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(schema) > 0 {
		if err := json.Unmarshal(schema, &t.Schema); err != nil {
			return nil, err
		}
	}
	return &t, t.Validate()
}

func (r *AuthorizationDetailTypeRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.AuthorizationDetailType, error) {
	rows, err := r.Pool.Query(ctx, authorizationDetailTypeSelect+" WHERE tenant_id=$1 ORDER BY type", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.AuthorizationDetailType{}
	for rows.Next() {
		t, err := scanAuthorizationDetailType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *AuthorizationDetailTypeRepository) FindByType(ctx context.Context, tenantID, detailType string) (*spec.AuthorizationDetailType, error) {
	return scanAuthorizationDetailType(r.Pool.QueryRow(ctx,
		authorizationDetailTypeSelect+" WHERE tenant_id=$1 AND type=$2", tenantID, detailType))
}

func (r *AuthorizationDetailTypeRepository) Save(ctx context.Context, t *spec.AuthorizationDetailType) error {
	schema, err := json.Marshal(t.Schema)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO authorization_detail_types (tenant_id,type,description,schema,display_template,state,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id,type) DO UPDATE SET description=EXCLUDED.description,schema=EXCLUDED.schema,
 display_template=EXCLUDED.display_template,state=EXCLUDED.state,updated_at=EXCLUDED.updated_at`,
		t.TenantID, t.Type, t.Description, schema, t.DisplayTemplate, t.State, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *AuthorizationDetailTypeRepository) Delete(ctx context.Context, tenantID, detailType string) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM authorization_detail_types WHERE tenant_id=$1 AND type=$2", tenantID, detailType)
	return err
}
