package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
)

func TestEnsureDefaultAndRejectDefaultDisable(t *testing.T) {
	repo := memory.NewTenantRepository()
	now := time.Now().UTC()
	if err := EnsureDefault(context.Background(), repo, now); err != nil {
		t.Fatal(err)
	}
	tenant, err := repo.FindByID(context.Background(), spec.DefaultTenantID)
	if err != nil {
		t.Fatal(err)
	}
	if tenant == nil || tenant.Status != spec.TenantStatusActive {
		t.Fatalf("default tenant = %#v", tenant)
	}
	if _, err := SetDisabled(
		context.Background(), repo, spec.DefaultTenantID, true, now,
	); !errors.Is(err, ErrDefaultTenant) {
		t.Fatalf("disable default error = %v", err)
	}
}

func TestTenantLifecycle(t *testing.T) {
	repo := memory.NewTenantRepository()
	tenant, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusActive {
		t.Fatalf("status = %s", tenant.Status)
	}
	tenant, err = SetDisabled(context.Background(), repo, "acme", true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusDisabled || tenant.DisabledAt == nil {
		t.Fatalf("disabled tenant = %#v", tenant)
	}
	tenant, err = SetDisabled(context.Background(), repo, "acme", false, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if tenant.Status != spec.TenantStatusActive || tenant.DisabledAt != nil {
		t.Fatalf("enabled tenant = %#v", tenant)
	}
}

func TestUpdateAppliesDisplayNameAndPolicyOverride(t *testing.T) {
	repo := memory.NewTenantRepository()
	if _, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	floor := PolicyFloor{MinLength: 12, MaxLength: 128, HistoryDepth: 5}
	newName := "Acme Inc."
	minLen := 16
	historyDepth := 10
	updated, err := Update(context.Background(), repo, "acme", UpdateInput{
		DisplayName: &newName,
		PasswordPolicyOverride: &spec.PasswordPolicyOverride{
			MinLength: &minLen, HistoryDepth: &historyDepth,
		},
	}, floor, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != newName {
		t.Fatalf("display_name = %q", updated.DisplayName)
	}
	if updated.PasswordPolicyOverride == nil ||
		updated.PasswordPolicyOverride.MinLength == nil ||
		*updated.PasswordPolicyOverride.MinLength != minLen {
		t.Fatalf("override = %#v", updated.PasswordPolicyOverride)
	}
	if updated.PasswordPolicyOverride.MaxLength != nil {
		t.Fatalf("max_length should remain unset: %#v", updated.PasswordPolicyOverride)
	}
}

func TestUpdateRejectsWeakerPolicyOverride(t *testing.T) {
	floor := PolicyFloor{MinLength: 12, MaxLength: 128, HistoryDepth: 5}
	cases := []struct {
		name     string
		override spec.PasswordPolicyOverride
	}{
		{"shorter min_length", spec.PasswordPolicyOverride{MinLength: ptrInt(8)}},
		{"longer max_length", spec.PasswordPolicyOverride{MaxLength: ptrInt(256)}},
		{"shorter history_depth", spec.PasswordPolicyOverride{HistoryDepth: ptrInt(2)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := memory.NewTenantRepository()
			if _, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			_, err := Update(context.Background(), repo, "acme", UpdateInput{
				PasswordPolicyOverride: &tc.override,
			}, floor, time.Now().UTC())
			if !errors.Is(err, ErrPolicyOverrideWeaker) {
				t.Fatalf("err = %v, want ErrPolicyOverrideWeaker", err)
			}
		})
	}
}

func TestUpdatePreservesUnsetFields(t *testing.T) {
	repo := memory.NewTenantRepository()
	if _, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	floor := PolicyFloor{MinLength: 12, MaxLength: 128, HistoryDepth: 5}
	minLen := 16
	if _, err := Update(context.Background(), repo, "acme", UpdateInput{
		PasswordPolicyOverride: &spec.PasswordPolicyOverride{MinLength: &minLen},
	}, floor, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// 後続の display_name 単独更新で override が消えないこと。
	newName := "Acme Renamed"
	updated, err := Update(context.Background(), repo, "acme", UpdateInput{
		DisplayName: &newName,
	}, floor, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if updated.DisplayName != newName {
		t.Fatalf("display_name = %q", updated.DisplayName)
	}
	if updated.PasswordPolicyOverride == nil ||
		updated.PasswordPolicyOverride.MinLength == nil ||
		*updated.PasswordPolicyOverride.MinLength != minLen {
		t.Fatalf("override lost: %#v", updated.PasswordPolicyOverride)
	}
}

func TestUpdateClearsOverrideWhenAllFieldsZero(t *testing.T) {
	repo := memory.NewTenantRepository()
	if _, err := Create(context.Background(), repo, "acme", "Acme", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	floor := PolicyFloor{MinLength: 12, MaxLength: 128, HistoryDepth: 5}
	// まず override を設定。
	minLen := 20
	if _, err := Update(context.Background(), repo, "acme", UpdateInput{
		PasswordPolicyOverride: &spec.PasswordPolicyOverride{MinLength: &minLen},
	}, floor, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// 空 override で送ると global default 継承に戻る。
	updated, err := Update(context.Background(), repo, "acme", UpdateInput{
		PasswordPolicyOverride: &spec.PasswordPolicyOverride{},
	}, floor, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if updated.PasswordPolicyOverride != nil {
		t.Fatalf("override should be cleared: %#v", updated.PasswordPolicyOverride)
	}
}

func ptrInt(v int) *int { return &v }
