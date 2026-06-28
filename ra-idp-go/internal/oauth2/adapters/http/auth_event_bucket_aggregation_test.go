package http

// wi-20 スライス 3: 閾値超過 (throttle Locked) のログイン失敗は個別イベントへ
// 落とさず 5 分窓の bucket に集約する。recordLoginFailure の契約を直接検証する:
//   - bucket store がある構成では aggregated=true を返し、窓の最初の 1 件だけ
//     AuthenticationEventAggregated を emit する (後続は count に積むだけ)。
//   - bucket store が無い構成では aggregated=false を返し、呼び出し側が従来どおり
//     個別の AuthenticationFailed を残せる。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type fakeLockedThrottle struct{}

func (fakeLockedThrottle) TryAcquire(
	context.Context, authnports.LoginThrottleKind, string, time.Time,
) (authnports.LoginThrottleResult, error) {
	return authnports.LoginThrottleResult{Allowed: true}, nil
}

func (fakeLockedThrottle) RecordFailure(
	context.Context, authnports.LoginThrottleKind, string, time.Time,
) (authnports.LoginThrottleResult, error) {
	return authnports.LoginThrottleResult{Allowed: false, Locked: true, RetryAfterSeconds: 900}, nil
}

func (fakeLockedThrottle) RecordSuccess(context.Context, authnports.LoginThrottleKind, string) error {
	return nil
}

func countEventType(events []spec.DomainEvent, typ string) int {
	n := 0
	for _, e := range events {
		if e.EventType() == typ {
			n++
		}
	}
	return n
}

// driveRecordLoginFailure は echo route 経由で recordLoginFailure を回数分呼び、
// 最後の aggregated 値を返す (account key だけ渡し IP は空にして 1 key に固定する)。
func driveRecordLoginFailure(t *testing.T, d core.Deps, username string, times int) (bool, []spec.DomainEvent) {
	t.Helper()
	var emitted []spec.DomainEvent
	d.Emit = func(e spec.DomainEvent) { emitted = append(emitted, e) }
	e := echo.New()
	lastAggregated := false
	e.POST("/x", func(c *echo.Context) error {
		agg, err := (Deps{&d}).recordLoginFailure(c, username, "")
		lastAggregated = agg
		return err
	})
	for i := range times {
		req := httptest.NewRequest(http.MethodPost, "/x", http.NoBody)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("call #%d status=%d", i, rec.Code)
		}
	}
	return lastAggregated, emitted
}

func TestRecordLoginFailureAggregatesWhenLocked(t *testing.T) {
	d := core.Deps{
		LoginAttemptThrottle: fakeLockedThrottle{},
		AuthEventBucketStore: memory.NewAuthEventBucketStore(),
	}
	aggregated, emitted := driveRecordLoginFailure(t, d, "alice", 3)
	if !aggregated {
		t.Fatalf("expected aggregated=true when locked with bucket store")
	}
	// 同一窓なので集約イベントは 1 件だけ。LoginThrottled は毎回出る。
	if got := countEventType(emitted, "AuthenticationEventAggregated"); got != 1 {
		t.Fatalf("expected 1 AuthenticationEventAggregated, got %d", got)
	}
	if got := countEventType(emitted, "LoginThrottled"); got != 3 {
		t.Fatalf("expected 3 LoginThrottled, got %d", got)
	}
	// 集約に切り替わったので個別の AuthenticationFailed は出さない (呼び出し側で抑制)。
	if got := countEventType(emitted, "AuthenticationFailed"); got != 0 {
		t.Fatalf("expected 0 AuthenticationFailed, got %d", got)
	}
}

func TestRecordLoginFailureWithoutBucketStoreDoesNotAggregate(t *testing.T) {
	d := core.Deps{LoginAttemptThrottle: fakeLockedThrottle{}}
	aggregated, emitted := driveRecordLoginFailure(t, d, "alice", 2)
	if aggregated {
		t.Fatalf("expected aggregated=false without bucket store")
	}
	if got := countEventType(emitted, "AuthenticationEventAggregated"); got != 0 {
		t.Fatalf("expected 0 AuthenticationEventAggregated, got %d", got)
	}
}
