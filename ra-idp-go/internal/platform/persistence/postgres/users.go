package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// UserRepository (Authentication)
type UserRepository struct{ Pool *pgxpool.Pool }

// notDeleted は削除済みユーザを除外する述語。削除状態は lifecycle.status に統合
// した (ADR-039)。status 未設定 (NULL) は active 扱いなので残す。
const notDeleted = " AND (lifecycle->>'status' IS DISTINCT FROM 'deleted')"

func (r *UserRepository) FindBySub(ctx context.Context, sub string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE sub=$1"+notDeleted, sub))
}

func (r *UserRepository) FindBySubIncludingDeleted(ctx context.Context, sub string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE sub=$1", sub))
}

func (r *UserRepository) FindByUsername(ctx context.Context, tenantID, username string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE tenant_id=$1 AND preferred_username=$2"+notDeleted, tenantID, username))
}

func (r *UserRepository) FindByEmail(ctx context.Context, tenantID, email string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(
		ctx,
		userSelect+" WHERE tenant_id=$1 AND lower(email)=lower($2)"+notDeleted+" LIMIT 1",
		tenantID, email,
	))
}

func (r *UserRepository) FindAll(ctx context.Context, tenantID string) ([]*spec.User, error) {
	rows, err := r.Pool.Query(ctx, userSelect+" WHERE tenant_id=$1"+notDeleted+" ORDER BY preferred_username", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*spec.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) Save(ctx context.Context, u *spec.User) error {
	// lifecycle / attributes は JSONB に格納する (ADR-039)。多値属性は本 PR では
	// 単一カラムで持ち、検索が要るようになった段階で別テーブル化する。
	lifecycle, err := json.Marshal(u.Lifecycle)
	if err != nil {
		return err
	}
	attributes, err := json.Marshal(u.Attributes)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO users (sub,tenant_id,preferred_username,password_hash,name,given_name,family_name,email,
 email_verified,mfa_enrolled,roles,lifecycle,attributes,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (sub) DO UPDATE SET preferred_username=EXCLUDED.preferred_username,
 password_hash=EXCLUDED.password_hash,name=EXCLUDED.name,given_name=EXCLUDED.given_name,
 family_name=EXCLUDED.family_name,email=EXCLUDED.email,email_verified=EXCLUDED.email_verified,
 mfa_enrolled=EXCLUDED.mfa_enrolled,roles=EXCLUDED.roles,lifecycle=EXCLUDED.lifecycle,
 attributes=EXCLUDED.attributes,updated_at=EXCLUDED.updated_at`,
		u.Sub, u.TenantID, u.PreferredUsername, u.PasswordHash, u.Name, u.GivenName, u.FamilyName, u.Email,
		u.EmailVerified, u.MfaEnrolled, u.Roles, lifecycle, attributes, u.CreatedAt, u.UpdatedAt)
	return err
}

const userSelect = `SELECT sub,tenant_id,preferred_username,password_hash,name,given_name,family_name,email,
email_verified,mfa_enrolled,roles,lifecycle,attributes,created_at,updated_at FROM users`

func scanUser(row rowScanner) (*spec.User, error) {
	var u spec.User
	var lifecycle, attributes []byte
	err := row.Scan(&u.Sub, &u.TenantID, &u.PreferredUsername, &u.PasswordHash, &u.Name, &u.GivenName,
		&u.FamilyName, &u.Email, &u.EmailVerified, &u.MfaEnrolled, &u.Roles, &lifecycle, &attributes,
		&u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(lifecycle) > 0 {
		if err := json.Unmarshal(lifecycle, &u.Lifecycle); err != nil {
			return nil, err
		}
	}
	if len(attributes) > 0 {
		if err := json.Unmarshal(attributes, &u.Attributes); err != nil {
			return nil, err
		}
	}
	return &u, u.Validate()
}
