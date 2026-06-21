package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authports "ra-idp-go/internal/authentication/ports"
)

// EmailChangeTokenStore (Authentication)
type EmailChangeTokenStore struct{ Pool *pgxpool.Pool }

func (s *EmailChangeTokenStore) Save(
	ctx context.Context,
	record authports.EmailChangeTokenRecord,
) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "DELETE FROM email_change_tokens WHERE sub=$1", record.Sub); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO email_change_tokens
(token_hash,sub,new_email,created_at,expires_at) VALUES ($1,$2,$3,$4,$5)`,
		record.TokenHash, record.Sub, record.NewEmail, record.CreatedAt, record.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *EmailChangeTokenStore) Consume(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*authports.EmailChangeTokenRecord, error) {
	var record authports.EmailChangeTokenRecord
	err := s.Pool.QueryRow(ctx, `DELETE FROM email_change_tokens
WHERE token_hash=$1
RETURNING sub,token_hash,new_email,created_at,expires_at`, tokenHash).
		Scan(&record.Sub, &record.TokenHash, &record.NewEmail, &record.CreatedAt, &record.ExpiresAt)
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
