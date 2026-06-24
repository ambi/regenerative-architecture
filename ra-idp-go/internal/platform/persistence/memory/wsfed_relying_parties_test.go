package memory

import (
	"context"
	"testing"

	"ra-idp-go/internal/spec"
)

func TestWsFedRelyingPartyRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewWsFedRelyingPartyRepository()

	rp := &spec.WsFedRelyingParty{
		Wtrealm:   "urn:federation:MicrosoftOnline",
		ReplyURLs: []string{"https://login.microsoftonline.com/login.srf"},
	}
	repo.Seed(rp) // tenant_id 未設定 → default に正規化される。

	got, err := repo.FindByWtrealm(ctx, spec.DefaultTenantID, "urn:federation:MicrosoftOnline")
	if err != nil {
		t.Fatalf("FindByWtrealm: %v", err)
	}
	if got == nil || got.TenantID != spec.DefaultTenantID {
		t.Fatalf("expected RP under default tenant, got %+v", got)
	}

	// クローンが返り、内部状態が共有されないこと。
	got.ReplyURLs[0] = "https://evil.example"
	again, _ := repo.FindByWtrealm(ctx, spec.DefaultTenantID, "urn:federation:MicrosoftOnline")
	if again.ReplyURLs[0] != "https://login.microsoftonline.com/login.srf" {
		t.Fatal("repository returned shared slice; mutation leaked")
	}

	missing, err := repo.FindByWtrealm(ctx, spec.DefaultTenantID, "urn:unknown")
	if err != nil {
		t.Fatalf("FindByWtrealm(unknown): %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for unknown wtrealm, got %+v", missing)
	}

	list, err := repo.ListByTenant(ctx, spec.DefaultTenantID)
	if err != nil {
		t.Fatalf("ListByTenant: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListByTenant = %d entries, want 1", len(list))
	}
}
