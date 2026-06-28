package usecases_test

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/authentication/usecases"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/infrastructure/persistence/memory"
)

func TestListSignInActivityFiltersBySubTenantAndType(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAuditEventStore(0)
	base := time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)

	// 古い順に追加する (memory store は挿入順の降順で返す)。
	add := func(rec *oauthports.AuditEventRecord) {
		if err := store.Append(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	add(&oauthports.AuditEventRecord{
		ID: "1", TenantID: "t1", Type: "UserAuthenticated", OccurredAt: base,
		Payload: map[string]any{"sub": "alice", "amr": []any{"pwd"}},
	})
	add(&oauthports.AuditEventRecord{
		ID: "2", TenantID: "t1", Type: "PasswordChanged", OccurredAt: base.Add(time.Minute),
		Payload: map[string]any{"sub": "alice"},
	})
	add(&oauthports.AuditEventRecord{
		ID: "3", TenantID: "t1", Type: "UserAuthenticated", OccurredAt: base.Add(2 * time.Minute),
		Payload: map[string]any{"sub": "bob", "amr": []any{"pwd"}},
	})
	add(&oauthports.AuditEventRecord{
		ID: "4", TenantID: "t2", Type: "UserAuthenticated", OccurredAt: base.Add(3 * time.Minute),
		Payload: map[string]any{"sub": "alice", "amr": []any{"pwd"}},
	})
	add(&oauthports.AuditEventRecord{
		ID: "5", TenantID: "t1", Type: "UserAuthenticated", OccurredAt: base.Add(4 * time.Minute),
		Payload: map[string]any{"sub": "alice", "amr": []any{"pwd", "otp"}},
	})

	got, err := usecases.ListSignInActivity(ctx, store, "t1", "alice", 0)
	if err != nil {
		t.Fatal(err)
	}
	// alice/t1 の UserAuthenticated は 2 件 (id 1, 5)。bob・t2・PasswordChanged は除外。
	if len(got) != 2 {
		t.Fatalf("len(got)=%d, want 2: %#v", len(got), got)
	}
	// 新しい順 (id 5 が先頭)。
	if !got[0].OccurredAt.Equal(base.Add(4 * time.Minute)) {
		t.Fatalf("first occurred_at=%v, want newest", got[0].OccurredAt)
	}
	if len(got[0].AMR) != 2 || got[0].AMR[0] != "pwd" || got[0].AMR[1] != "otp" {
		t.Fatalf("unexpected amr: %#v", got[0].AMR)
	}
}

func TestListSignInActivityClampsLimit(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAuditEventStore(0)
	base := time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)
	for i := range usecases.SignInActivityMaxLimit + 20 {
		if err := store.Append(ctx, &oauthports.AuditEventRecord{
			ID: string(rune('a' + (i % 26))), TenantID: "t1", Type: "UserAuthenticated",
			OccurredAt: base.Add(time.Duration(i) * time.Minute),
			Payload:    map[string]any{"sub": "alice", "amr": []any{"pwd"}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := usecases.ListSignInActivity(ctx, store, "t1", "alice", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > usecases.SignInActivityMaxLimit {
		t.Fatalf("len(got)=%d exceeds max %d", len(got), usecases.SignInActivityMaxLimit)
	}
}

func TestListSignInActivityNilRepo(t *testing.T) {
	got, err := usecases.ListSignInActivity(context.Background(), nil, "t1", "alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got)=%d, want 0", len(got))
	}
}
