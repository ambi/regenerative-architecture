package ports

import "context"

type BreachedPasswordChecker interface {
	IsBreached(ctx context.Context, password string) bool
}
