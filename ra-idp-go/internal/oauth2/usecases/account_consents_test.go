package usecases_test

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

func accountConsentCtx() context.Context {
	return tenancy.WithTenant(
		context.Background(),
		&spec.Tenant{ID: spec.DefaultTenantID, Status: spec.TenantStatusActive},
		"http://idp.test", "",
	)
}

func saveConsent(t *testing.T, repo *memory.ConsentRepository, sub, client string, state spec.ConsentState) {
	t.Helper()
	now := time.Now().UTC()
	if err := repo.Save(accountConsentCtx(), &spec.Consent{
		TenantID: spec.DefaultTenantID, Sub: sub, ClientID: client, Scopes: []string{"openid"},
		State: state, GrantedAt: now, ExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestListConsentsForSubReturnsOnlyOwnGrantedConsents(t *testing.T) {
	ctx := accountConsentCtx()
	repo := memory.NewConsentRepository()
	saveConsent(t, repo, "user-alice", "app-1", spec.ConsentGranted)
	saveConsent(t, repo, "user-alice", "app-2", spec.ConsentRevoked)
	saveConsent(t, repo, "user-bob", "app-3", spec.ConsentGranted)

	got, err := usecases.ListConsentsForSub(ctx, usecases.ConsentDeps{ConsentRepo: repo}, "user-alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ClientID != "app-1" {
		t.Fatalf("expected only alice's granted app-1, got %#v", got)
	}
}

func TestRevokeConsentSelfMarksRevokedAndEmits(t *testing.T) {
	ctx := accountConsentCtx()
	repo := memory.NewConsentRepository()
	saveConsent(t, repo, "user-alice", "app-1", spec.ConsentGranted)

	var events []spec.DomainEvent
	deps := usecases.ConsentDeps{ConsentRepo: repo, Emit: func(e spec.DomainEvent) { events = append(events, e) }}
	if err := usecases.RevokeConsent(ctx, deps, "user-alice", "user-alice", "app-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, _ := usecases.ListConsentsForSub(ctx, deps, "user-alice")
	if len(got) != 0 {
		t.Fatalf("revoked consent must not appear in active list: %#v", got)
	}
	if len(events) != 1 || events[0].EventType() != "ConsentRevoked" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
