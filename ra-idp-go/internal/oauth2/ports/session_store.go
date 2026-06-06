package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type SessionStore interface {
	Save(ctx context.Context, s *spec.LoginSession) error
	Find(ctx context.Context, sessionID string) (*spec.LoginSession, error)
	Delete(ctx context.Context, sessionID string) error
}
