package ports

import (
	"context"
	"time"
)

type AccessTokenDenylist interface {
	Add(ctx context.Context, jti string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
