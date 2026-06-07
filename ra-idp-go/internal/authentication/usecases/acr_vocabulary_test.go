package usecases

import "testing"

func TestDeriveACR(t *testing.T) {
	if got := DeriveACR([]string{"pwd"}); got != ACRPassword {
		t.Fatalf("password ACR=%q", got)
	}
	if got := DeriveACR([]string{"pwd", "otp"}); got != ACRMFA {
		t.Fatalf("MFA ACR=%q", got)
	}
}

func TestACRSatisfies(t *testing.T) {
	if !ACRSatisfies(ACRMFA, ACRPassword) {
		t.Fatal("MFA should satisfy password ACR")
	}
	if ACRSatisfies(ACRPassword, ACRMFA) {
		t.Fatal("password ACR must not satisfy MFA")
	}
}
