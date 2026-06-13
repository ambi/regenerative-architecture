package ports

import (
	"context"
	"time"
)

type PasswordResetTokenRecord struct {
	Sub       string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type PasswordResetTokenStore interface {
	Save(ctx context.Context, record PasswordResetTokenRecord) error
	Consume(ctx context.Context, tokenHash string, now time.Time) (*PasswordResetTokenRecord, error)
}
