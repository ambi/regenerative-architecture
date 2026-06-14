package ports

import (
	"context"
	"time"
)

type LoginThrottleKind string

const (
	LoginThrottleAccount LoginThrottleKind = "account"
	LoginThrottleIP      LoginThrottleKind = "ip"
)

type LoginThrottleResult struct {
	Allowed           bool
	Locked            bool
	RetryAfterSeconds int
}

type LoginAttemptThrottle interface {
	TryAcquire(ctx context.Context, kind LoginThrottleKind, key string, now time.Time) (LoginThrottleResult, error)
	RecordFailure(ctx context.Context, kind LoginThrottleKind, key string, now time.Time) (LoginThrottleResult, error)
	RecordSuccess(ctx context.Context, kind LoginThrottleKind, key string) error
}
