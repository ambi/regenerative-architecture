package memory

import (
	"context"
	"math"
	"sync"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

type LoginThrottleConfig struct {
	MaxFailures    int
	WindowSeconds  int
	LockoutSeconds int
}

type LoginThrottleConfigs struct {
	Account LoginThrottleConfig
	IP      LoginThrottleConfig
}

type loginCounter struct {
	failures  int
	expiresAt time.Time
}

type LoginAttemptThrottle struct {
	mu       sync.Mutex
	configs  LoginThrottleConfigs
	counters map[string]loginCounter
	locks    map[string]time.Time
}

func NewLoginAttemptThrottle(configs LoginThrottleConfigs) *LoginAttemptThrottle {
	return &LoginAttemptThrottle{
		configs: configs, counters: map[string]loginCounter{}, locks: map[string]time.Time{},
	}
}

func (t *LoginAttemptThrottle) TryAcquire(
	_ context.Context,
	kind authports.LoginThrottleKind,
	key string,
	now time.Time,
) (authports.LoginThrottleResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	lockKey := throttleKey(kind, key)
	expiresAt, ok := t.locks[lockKey]
	if !ok {
		return authports.LoginThrottleResult{Allowed: true}, nil
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		delete(t.locks, lockKey)
		return authports.LoginThrottleResult{Allowed: true}, nil
	}
	return authports.LoginThrottleResult{
		Allowed: false, Locked: true,
		RetryAfterSeconds: int(math.Ceil(remaining.Seconds())),
	}, nil
}

func (t *LoginAttemptThrottle) RecordFailure(
	_ context.Context,
	kind authports.LoginThrottleKind,
	key string,
	now time.Time,
) (authports.LoginThrottleResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	config := t.config(kind)
	counterKey := throttleKey(kind, key)
	counter, ok := t.counters[counterKey]
	if !ok || !now.Before(counter.expiresAt) {
		counter = loginCounter{failures: 1, expiresAt: now.Add(time.Duration(config.WindowSeconds) * time.Second)}
	} else {
		counter.failures++
	}
	t.counters[counterKey] = counter
	if counter.failures < config.MaxFailures {
		return authports.LoginThrottleResult{Allowed: true}, nil
	}
	delete(t.counters, counterKey)
	t.locks[counterKey] = now.Add(time.Duration(config.LockoutSeconds) * time.Second)
	return authports.LoginThrottleResult{
		Allowed: false, Locked: true, RetryAfterSeconds: config.LockoutSeconds,
	}, nil
}

func (t *LoginAttemptThrottle) RecordSuccess(
	_ context.Context,
	kind authports.LoginThrottleKind,
	key string,
) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	key = throttleKey(kind, key)
	delete(t.counters, key)
	delete(t.locks, key)
	return nil
}

func (t *LoginAttemptThrottle) config(kind authports.LoginThrottleKind) LoginThrottleConfig {
	if kind == authports.LoginThrottleIP {
		return t.configs.IP
	}
	return t.configs.Account
}

func throttleKey(kind authports.LoginThrottleKind, key string) string {
	return string(kind) + ":" + key
}
