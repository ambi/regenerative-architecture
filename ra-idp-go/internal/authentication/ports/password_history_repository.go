package ports

import (
	"context"
	"time"
)

type PasswordHistoryEntry struct {
	Encoded   string
	CreatedAt time.Time
}

type PasswordHistoryRepository interface {
	Recent(ctx context.Context, sub string, depth int) ([]PasswordHistoryEntry, error)
	Add(ctx context.Context, sub, encoded string, now time.Time) error
}
