package usecases_test

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

func daysAgo(now time.Time, d int) time.Time {
	return now.Add(-time.Duration(d) * 24 * time.Hour)
}

func seedAudit(t *testing.T, store *memory.AuditEventStore, id, eventType string, at time.Time) {
	t.Helper()
	if err := store.Append(context.Background(), &oauthports.AuditEventRecord{
		ID: id, TenantID: "default", Type: eventType, OccurredAt: at,
		Payload: map[string]any{"tenantId": "default"},
	}); err != nil {
		t.Fatal(err)
	}
}

func remainingAuditIDs(t *testing.T, store *memory.AuditEventStore) map[string]bool {
	t.Helper()
	recs, err := store.List(context.Background(), oauthports.AuditEventQuery{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range recs {
		out[r.ID] = true
	}
	return out
}

func TestRetentionSweepDeletesByTypeBoundaries(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := memory.NewAuditEventStore(0)

	// 失敗詳細 30 日: 29 日前は残り 31 日前は消える。
	seedAudit(t, store, "fail-29", (&spec.AuthenticationFailed{}).EventType(), daysAgo(now, 29))
	seedAudit(t, store, "fail-31", (&spec.AuthenticationFailed{}).EventType(), daysAgo(now, 31))
	// 成功 365 日: 364 日前は残り 366 日前は消える。
	seedAudit(t, store, "ok-364", (&spec.UserAuthenticated{}).EventType(), daysAgo(now, 364))
	seedAudit(t, store, "ok-366", (&spec.UserAuthenticated{}).EventType(), daysAgo(now, 366))
	// セッション / bucket 90 日: 89 日前は残り 91 日前は消える。
	seedAudit(t, store, "sess-89", (&spec.SessionStarted{}).EventType(), daysAgo(now, 89))
	seedAudit(t, store, "sess-91", (&spec.SessionStarted{}).EventType(), daysAgo(now, 91))
	seedAudit(t, store, "agg-91", (&spec.AuthenticationEventAggregated{}).EventType(), daysAgo(now, 91))
	// impersonation: cap 未設定なら無期限保持 (400 日前でも残る)。
	seedAudit(t, store, "imp-400", (&spec.SessionImpersonationStarted{}).EventType(), daysAgo(now, 400))

	res, err := usecases.RunRetentionSweep(ctx, store, nil, usecases.DefaultRetentionPolicy(), now)
	if err != nil {
		t.Fatal(err)
	}
	if res.AuditEvents != 4 {
		t.Fatalf("deleted audit=%d, want 4", res.AuditEvents)
	}
	got := remainingAuditIDs(t, store)
	wantKept := []string{"fail-29", "ok-364", "sess-89", "imp-400"}
	wantGone := []string{"fail-31", "ok-366", "sess-91", "agg-91"}
	for _, id := range wantKept {
		if !got[id] {
			t.Errorf("%s should be kept", id)
		}
	}
	for _, id := range wantGone {
		if got[id] {
			t.Errorf("%s should be deleted", id)
		}
	}
}

func TestRetentionSweepGlobalCapShortensAndDeletesImpersonation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := memory.NewAuditEventStore(0)
	// global cap 30 日: 成功も impersonation も 31 日前は消える。
	seedAudit(t, store, "ok-31", (&spec.UserAuthenticated{}).EventType(), daysAgo(now, 31))
	seedAudit(t, store, "ok-29", (&spec.UserAuthenticated{}).EventType(), daysAgo(now, 29))
	seedAudit(t, store, "imp-31", (&spec.SessionImpersonationStarted{}).EventType(), daysAgo(now, 31))

	policy := usecases.DefaultRetentionPolicy()
	policy.MaxDays = 30
	if _, err := usecases.RunRetentionSweep(ctx, store, nil, policy, now); err != nil {
		t.Fatal(err)
	}
	got := remainingAuditIDs(t, store)
	if !got["ok-29"] {
		t.Error("ok-29 should be kept under cap=30")
	}
	if got["ok-31"] {
		t.Error("ok-31 should be deleted under cap=30")
	}
	if got["imp-31"] {
		t.Error("imp-31 should be deleted: global cap shortens impersonation")
	}
}

func TestRetentionSweepDeletesOldBuckets(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	store := memory.NewAuthEventBucketStore()
	// 91 日前の窓と直近の窓を作る。
	if _, err := store.Record(ctx, "failed_login", "default", "old-key", daysAgo(now, 91)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record(ctx, "failed_login", "default", "fresh-key", now); err != nil {
		t.Fatal(err)
	}
	res, err := usecases.RunRetentionSweep(ctx, nil, store, usecases.DefaultRetentionPolicy(), now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Buckets != 1 {
		t.Fatalf("deleted buckets=%d, want 1", res.Buckets)
	}
	buckets, err := store.List(ctx, "default", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 1 || buckets[0].KeyHash != "fresh-key" {
		t.Fatalf("remaining buckets=%#v, want only fresh-key", buckets)
	}
}
