package memory

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/ports"
)

func newAuditEvent(t *testing.T, tenantID, typ string, occurredAt time.Time, sub string) *ports.AuditEventRecord {
	t.Helper()
	return &ports.AuditEventRecord{
		ID:         tenantID + ":" + typ + ":" + sub + ":" + occurredAt.Format(time.RFC3339Nano),
		TenantID:   tenantID,
		Type:       typ,
		OccurredAt: occurredAt,
		Payload:    map[string]any{"sub": sub, "tenantId": tenantID},
	}
}

func TestAuditEventStoreFiltersAndOrders(t *testing.T) {
	store := NewAuditEventStore(0)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, ev := range []*ports.AuditEventRecord{
		newAuditEvent(t, "acme", "UserAuthenticated", base, "alice"),
		newAuditEvent(t, "acme", "AccessTokenIssued", base.Add(2*time.Second), "alice"),
		newAuditEvent(t, "default", "UserAuthenticated", base.Add(3*time.Second), "bob"),
		newAuditEvent(t, "acme", "UserAuthenticated", base.Add(4*time.Second), "carol"),
	} {
		if err := store.Append(context.Background(), ev); err != nil {
			t.Fatalf("append #%d: %v", i, err)
		}
	}

	// 全テナントを暗黙に閉じた acme フィルタは acme のみ降順で返す。
	out, err := store.List(context.Background(), ports.AuditEventQuery{TenantID: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 acme events, got %d", len(out))
	}
	if out[0].OccurredAt.Before(out[len(out)-1].OccurredAt) {
		t.Fatalf("results must be in descending OccurredAt order: %+v", out)
	}

	// type フィルタ + sub フィルタの結合。
	filtered, err := store.List(context.Background(), ports.AuditEventQuery{
		TenantID: "acme", Type: "UserAuthenticated", Sub: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Payload["sub"] != "alice" {
		t.Fatalf("filter mismatch: %+v", filtered)
	}

	// AllTenants=true は default を含めて全件を返す。
	all, err := store.List(context.Background(), ports.AuditEventQuery{AllTenants: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("AllTenants must include default events, got %d", len(all))
	}

	// After フィルタは境界を含む (BeforeForce: rec.Before(After) を弾く)。
	after, err := store.List(context.Background(), ports.AuditEventQuery{
		TenantID: "acme", After: base.Add(3 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Payload["sub"] != "carol" {
		t.Fatalf("After filter: %+v", after)
	}
}

func TestAuditEventStoreEvictsBeyondCapacity(t *testing.T) {
	// maxEvents=3 で 4 件追加すると一番古い 1 件が落ちる。byID も同期する。
	store := NewAuditEventStore(3)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	first := newAuditEvent(t, "acme", "X", base, "a")
	for _, ev := range []*ports.AuditEventRecord{
		first,
		newAuditEvent(t, "acme", "X", base.Add(time.Second), "b"),
		newAuditEvent(t, "acme", "X", base.Add(2*time.Second), "c"),
		newAuditEvent(t, "acme", "X", base.Add(3*time.Second), "d"),
	} {
		if err := store.Append(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := store.FindByID(context.Background(), first.ID); got != nil {
		t.Fatal("oldest event must be evicted")
	}
	out, err := store.List(context.Background(), ports.AuditEventQuery{TenantID: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("capacity not enforced: len=%d", len(out))
	}
}

func TestAuditEventStoreLimitCapsAt1000(t *testing.T) {
	store := NewAuditEventStore(0)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 1100; i++ {
		_ = store.Append(context.Background(), &ports.AuditEventRecord{
			ID: "e" + time.Duration(i).String(), TenantID: "acme", Type: "X",
			OccurredAt: base.Add(time.Duration(i) * time.Second),
			Payload:    map[string]any{"sub": "u"},
		})
	}
	out, _ := store.List(context.Background(), ports.AuditEventQuery{TenantID: "acme", Limit: 10000})
	if len(out) != 1000 {
		t.Fatalf("limit must cap at 1000, got %d", len(out))
	}
}
