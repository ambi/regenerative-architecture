package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestPKCES256Verifies(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if !VerifyPKCES256(verifier, challenge) {
		t.Fatal("expected matching verifier/challenge to verify")
	}
}

func TestPKCES256RejectsMismatch(t *testing.T) {
	if VerifyPKCES256("verifier", "not-its-hash") {
		t.Fatal("expected mismatch to fail")
	}
}
