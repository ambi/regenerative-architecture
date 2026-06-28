package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	authnports "ra-idp-go/internal/authentication/ports"
)

// PasswordHistoryRepository (Authentication)
type PasswordHistoryRepository struct{ Pool *pgxpool.Pool }

func (r *PasswordHistoryRepository) Recent(
	ctx context.Context,
	sub string,
	depth int,
) ([]authnports.PasswordHistoryEntry, error) {
	if depth <= 0 {
		return nil, nil
	}
	rows, err := r.Pool.Query(ctx, `SELECT encoded,created_at
FROM password_history
WHERE sub=$1
ORDER BY created_at DESC, id DESC
LIMIT $2`, sub, depth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authnports.PasswordHistoryEntry{}
	for rows.Next() {
		var entry authnports.PasswordHistoryEntry
		if err := rows.Scan(&entry.Encoded, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *PasswordHistoryRepository) Add(ctx context.Context, sub, encoded string, now time.Time) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO password_history (sub,encoded,created_at) VALUES ($1,$2,$3)`,
		sub, encoded, now)
	return err
}

func (r *PasswordHistoryRepository) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM password_history WHERE sub=$1", sub)
	return err
}
