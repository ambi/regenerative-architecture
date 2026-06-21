package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// RefreshTokenStore (OAuth2)
type RefreshTokenStore struct{ Pool *pgxpool.Pool }

func (s *RefreshTokenStore) FindByHash(ctx context.Context, hash string) (*spec.RefreshTokenRecord, error) {
	return scanRefresh(s.Pool.QueryRow(ctx, refreshSelect+" WHERE hash=$1", hash))
}

func (s *RefreshTokenStore) Save(ctx context.Context, rec *spec.RefreshTokenRecord) error {
	return insertRefresh(ctx, s.Pool, rec)
}

func (s *RefreshTokenStore) Rotate(ctx context.Context, parentID string, next *spec.RefreshTokenRecord) (*spec.RefreshTokenRecord, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var rotated, revoked bool
	err = tx.QueryRow(ctx, `SELECT rotated,revoked FROM refresh_tokens WHERE id=$1 FOR UPDATE`, parentID).
		Scan(&rotated, &revoked)
	if errors.Is(err, pgx.ErrNoRows) || rotated || revoked {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE refresh_tokens SET rotated=TRUE WHERE id=$1", parentID); err != nil {
		return nil, err
	}
	if err := insertRefresh(ctx, tx, next); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *RefreshTokenStore) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE refresh_tokens SET revoked=TRUE WHERE family_id=$1", familyID)
	return err
}

func (s *RefreshTokenStore) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE sub=$1", sub)
	return err
}

const refreshSelect = `SELECT id::text,tenant_id,hash,family_id::text,parent_id::text,client_id,sub,scopes,
issued_at,expires_at,absolute_expires_at,revoked,rotated,sender_constraint FROM refresh_tokens`

func scanRefresh(row rowScanner) (*spec.RefreshTokenRecord, error) {
	var rec spec.RefreshTokenRecord
	var parentID *string
	var scopes, constraint []byte
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.Hash, &rec.FamilyID, &parentID, &rec.ClientID, &rec.Sub,
		&scopes, &rec.IssuedAt, &rec.ExpiresAt, &rec.AbsoluteExpiresAt, &rec.Revoked,
		&rec.Rotated, &constraint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.ParentID = parentID
	if err := json.Unmarshal(scopes, &rec.Scopes); err != nil {
		return nil, err
	}
	if len(constraint) > 0 {
		if err := json.Unmarshal(constraint, &rec.SenderConstraint); err != nil {
			return nil, err
		}
	}
	return &rec, rec.Validate()
}

func insertRefresh(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, rec *spec.RefreshTokenRecord,
) error {
	scopes, _ := json.Marshal(rec.Scopes)
	constraint, _ := json.Marshal(rec.SenderConstraint)
	_, err := db.Exec(ctx, `INSERT INTO refresh_tokens
(id,tenant_id,hash,family_id,parent_id,client_id,sub,scopes,issued_at,expires_at,absolute_expires_at,
revoked,rotated,sender_constraint) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,'null')::jsonb)`,
		rec.ID, rec.TenantID, rec.Hash, rec.FamilyID, rec.ParentID, rec.ClientID, rec.Sub, string(scopes),
		rec.IssuedAt, rec.ExpiresAt, rec.AbsoluteExpiresAt, rec.Revoked, rec.Rotated, string(constraint))
	return err
}
