package usecases_test

import (
	"context"
	"testing"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/infrastructure/persistence/memory"
)

func TestListAuthEventBucketsProjectsAndScopesByTenant(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAuthEventBucketStore()
	base := time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)

	for _, rec := range []struct {
		tenant, key string
	}{
		{"acme", "k1"}, {"acme", "k1"}, {"other", "k1"},
	} {
		if _, err := store.Record(ctx, authnports.AuthEventBucketFailedLogin, rec.tenant, rec.key, base); err != nil {
			t.Fatal(err)
		}
	}

	views, err := usecases.ListAuthEventBuckets(ctx, store, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 acme bucket, got %d", len(views))
	}
	v := views[0]
	if v.Kind != string(authnports.AuthEventBucketFailedLogin) {
		t.Fatalf("unexpected kind %q", v.Kind)
	}
	if v.Count != 2 {
		t.Fatalf("expected count 2, got %d", v.Count)
	}
	if v.KeyHash != "k1" {
		t.Fatalf("unexpected keyHash %q", v.KeyHash)
	}
}

func TestListAuthEventBucketsNilStoreReturnsEmpty(t *testing.T) {
	views, err := usecases.ListAuthEventBuckets(context.Background(), nil, "acme", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 0 {
		t.Fatalf("expected empty, got %d", len(views))
	}
}
