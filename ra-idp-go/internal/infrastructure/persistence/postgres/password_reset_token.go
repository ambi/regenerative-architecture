package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authnports "ra-idp-go/internal/authentication/ports"
)

// PasswordResetTokenStore (Authentication)
type PasswordResetTokenStore struct{ Pool *pgxpool.Pool }

func (s *PasswordResetTokenStore) Save(
	ctx context.Context,
	record authnports.PasswordResetTokenRecord,
) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "DELETE FROM password_reset_tokens WHERE sub=$1", record.Sub); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO password_reset_tokens
(token_hash,sub,created_at,expires_at) VALUES ($1,$2,$3,$4)`,
		record.TokenHash, record.Sub, record.CreatedAt, record.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PasswordResetTokenStore) Consume(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*authnports.PasswordResetTokenRecord, error) {
	var record authnports.PasswordResetTokenRecord
	err := s.Pool.QueryRow(ctx, `DELETE FROM password_reset_tokens
WHERE token_hash=$1
RETURNING sub,token_hash,created_at,expires_at`, tokenHash).
		Scan(&record.Sub, &record.TokenHash, &record.CreatedAt, &record.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !now.Before(record.ExpiresAt) {
		return nil, nil
	}
	return &record, nil
}
