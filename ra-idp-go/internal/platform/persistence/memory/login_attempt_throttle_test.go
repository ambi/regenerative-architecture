package memory

import (
	"context"
	"testing"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

func TestLoginAttemptThrottleLocksAndExpires(t *testing.T) {
	throttle := NewLoginAttemptThrottle(LoginThrottleConfigs{
		Account: LoginThrottleConfig{MaxFailures: 3, WindowSeconds: 60, LockoutSeconds: 120},
		IP:      LoginThrottleConfig{MaxFailures: 5, WindowSeconds: 60, LockoutSeconds: 60},
	})
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		result, err := throttle.RecordFailure(ctx, authports.LoginThrottleAccount, "alice", now)
		if err != nil || result.Locked {
			t.Fatalf("failure %d: result=%+v err=%v", i+1, result, err)
		}
	}
	result, err := throttle.RecordFailure(ctx, authports.LoginThrottleAccount, "alice", now)
	if err != nil || !result.Locked || result.RetryAfterSeconds != 120 {
		t.Fatalf("lock result=%+v err=%v", result, err)
	}
	acquire, err := throttle.TryAcquire(ctx, authports.LoginThrottleAccount, "alice", now)
	if err != nil || acquire.Allowed || acquire.RetryAfterSeconds != 120 {
		t.Fatalf("acquire result=%+v err=%v", acquire, err)
	}
	acquire, err = throttle.TryAcquire(ctx, authports.LoginThrottleAccount, "alice", now.Add(121*time.Second))
	if err != nil || !acquire.Allowed {
		t.Fatalf("expired lock result=%+v err=%v", acquire, err)
	}
}

func TestLoginAttemptThrottleSuccessClearsAccountOnly(t *testing.T) {
	throttle := NewLoginAttemptThrottle(LoginThrottleConfigs{
		Account: LoginThrottleConfig{MaxFailures: 1, WindowSeconds: 60, LockoutSeconds: 120},
		IP:      LoginThrottleConfig{MaxFailures: 1, WindowSeconds: 60, LockoutSeconds: 120},
	})
	ctx := context.Background()
	now := time.Now().UTC()
	_, _ = throttle.RecordFailure(ctx, authports.LoginThrottleAccount, "alice", now)
	_, _ = throttle.RecordFailure(ctx, authports.LoginThrottleIP, "203.0.113.1", now)
	if err := throttle.RecordSuccess(ctx, authports.LoginThrottleAccount, "alice"); err != nil {
		t.Fatal(err)
	}
	account, _ := throttle.TryAcquire(ctx, authports.LoginThrottleAccount, "alice", now)
	ip, _ := throttle.TryAcquire(ctx, authports.LoginThrottleIP, "203.0.113.1", now)
	if !account.Allowed || ip.Allowed {
		t.Fatalf("account=%+v ip=%+v", account, ip)
	}
}
