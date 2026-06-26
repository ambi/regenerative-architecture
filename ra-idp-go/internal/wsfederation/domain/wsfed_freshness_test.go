package domain

import (
	"testing"
	"time"
)

func TestRequiresFreshAuth(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		wfresh   string
		authTime time.Time
		want     bool
	}{
		{"absent: no constraint", "", now.Add(-time.Hour), false},
		{"invalid: no constraint", "abc", now.Add(-time.Hour), false},
		{"negative: no constraint", "-5", now.Add(-time.Hour), false},
		{"wfresh=0 within grace: ok", "0", now.Add(-5 * time.Second), false},
		{"wfresh=0 stale session: re-auth", "0", now.Add(-10 * time.Minute), true},
		{"wfresh=10 fresh: ok", "10", now.Add(-5 * time.Minute), false},
		{"wfresh=10 stale: re-auth", "10", now.Add(-15 * time.Minute), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RequiresFreshAuth(tc.wfresh, tc.authTime, now); got != tc.want {
				t.Fatalf("RequiresFreshAuth(%q) = %v, want %v", tc.wfresh, got, tc.want)
			}
		})
	}
}

func TestResolveAuthnMethod(t *testing.T) {
	t.Run("no wauth reflects performed password", func(t *testing.T) {
		m, err := ResolveAuthnMethod("", []string{"pwd"})
		if err != nil || m != AuthnPassword {
			t.Fatalf("got (%v, %v), want (password, nil)", m, err)
		}
	})
	t.Run("no wauth, no amr is unspecified", func(t *testing.T) {
		m, err := ResolveAuthnMethod("", nil)
		if err != nil || m != AuthnUnspecified {
			t.Fatalf("got (%v, %v), want (unspecified, nil)", m, err)
		}
	})
	t.Run("password wauth satisfied by password", func(t *testing.T) {
		m, err := ResolveAuthnMethod("urn:oasis:names:tc:SAML:2.0:ac:classes:Password", []string{"pwd"})
		if err != nil || m != AuthnPassword {
			t.Fatalf("got (%v, %v), want (password, nil)", m, err)
		}
	})
	t.Run("password wauth not satisfied without password", func(t *testing.T) {
		if _, err := ResolveAuthnMethod("urn:oasis:names:tc:SAML:1.0:am:password", nil); err == nil {
			t.Fatal("expected error when password method requested but not performed")
		}
	})
	t.Run("integrated windows is fail-closed", func(t *testing.T) {
		if _, err := ResolveAuthnMethod("urn:federation:authentication:windows", []string{"pwd"}); err == nil {
			t.Fatal("expected error for unsupported integrated Windows auth")
		}
	})
	t.Run("unknown wauth treated as hint", func(t *testing.T) {
		m, err := ResolveAuthnMethod("urn:example:custom", []string{"pwd"})
		if err != nil || m != AuthnPassword {
			t.Fatalf("got (%v, %v), want (password, nil)", m, err)
		}
	})
}
