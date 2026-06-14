package spec

import "testing"

func TestResolvePasswordPolicy(t *testing.T) {
	scl, err := LoadSCL()
	if err != nil {
		t.Fatalf("load scl: %v", err)
	}
	defaults := PasswordPolicySnapshot{MinLength: 12, MaxLength: 128, HistoryDepth: 5}

	t.Run("nil tenant uses SCL global", func(t *testing.T) {
		snap := scl.ResolvePasswordPolicy(nil, defaults)
		if snap.MinLength == 0 || snap.MaxLength == 0 || snap.HistoryDepth == 0 {
			t.Fatalf("snapshot zero values: %+v", snap)
		}
	})

	t.Run("override only specified fields", func(t *testing.T) {
		minLength := 16
		tenant := &Tenant{
			PasswordPolicyOverride: &PasswordPolicyOverride{MinLength: &minLength},
		}
		base := scl.ResolvePasswordPolicy(nil, defaults)
		snap := scl.ResolvePasswordPolicy(tenant, defaults)
		if snap.MinLength != 16 {
			t.Fatalf("MinLength override not applied: %d", snap.MinLength)
		}
		if snap.MaxLength != base.MaxLength || snap.HistoryDepth != base.HistoryDepth {
			t.Fatalf("non-overridden fields drifted: %+v vs base %+v", snap, base)
		}
	})

	t.Run("zero or negative override ignored", func(t *testing.T) {
		zero := 0
		neg := -1
		tenant := &Tenant{
			PasswordPolicyOverride: &PasswordPolicyOverride{
				MinLength:    &zero,
				MaxLength:    &neg,
				HistoryDepth: &zero,
			},
		}
		base := scl.ResolvePasswordPolicy(nil, defaults)
		snap := scl.ResolvePasswordPolicy(tenant, defaults)
		if snap != base {
			t.Fatalf("guard rail breached: %+v vs base %+v", snap, base)
		}
	})
}
