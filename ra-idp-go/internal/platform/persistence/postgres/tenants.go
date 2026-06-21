package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// TenantRepository (Tenancy)
type TenantRepository struct{ Pool *pgxpool.Pool }

func (r *TenantRepository) FindByID(ctx context.Context, id string) (*spec.Tenant, error) {
	return scanTenant(r.Pool.QueryRow(ctx, tenantSelect+" WHERE id=$1", id))
}

func (r *TenantRepository) FindAll(ctx context.Context) ([]*spec.Tenant, error) {
	rows, err := r.Pool.Query(ctx, tenantSelect+" ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Tenant{}
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tenant)
	}
	return out, rows.Err()
}

func (r *TenantRepository) Save(ctx context.Context, tenant *spec.Tenant) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO tenants
(id,display_name,status,created_at,updated_at,disabled_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name,
status=EXCLUDED.status,updated_at=EXCLUDED.updated_at,disabled_at=EXCLUDED.disabled_at`,
		tenant.ID, tenant.DisplayName, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt, tenant.DisabledAt)
	return err
}

const tenantSelect = `SELECT id,display_name,status,created_at,updated_at,disabled_at FROM tenants`

func scanTenant(row rowScanner) (*spec.Tenant, error) {
	var tenant spec.Tenant
	err := row.Scan(&tenant.ID, &tenant.DisplayName, &tenant.Status, &tenant.CreatedAt,
		&tenant.UpdatedAt, &tenant.DisabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tenant, tenant.Validate()
}
