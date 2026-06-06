package crypto

import (
	"strings"
	"testing"
)

func TestHashProducesPHCFormat(t *testing.T) {
	h := NewArgon2idPasswordHasher()
	encoded, err := h.Hash("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected PHC prefix: %s", encoded)
	}
	if strings.Count(encoded, "$") != 5 {
		t.Fatalf("expected 5 '$' separators, got %d in %s", strings.Count(encoded, "$"), encoded)
	}
}

func TestVerifyAcceptsMatchingPassword(t *testing.T) {
	h := NewArgon2idPasswordHasher()
	encoded, err := h.Hash("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	ok, err := h.Verify("correct-horse-battery-staple", encoded)
	if err != nil {
		t.Fatalf("verify errored: %v", err)
	}
	if !ok {
		t.Fatal("expected verify to accept the original password")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	h := NewArgon2idPasswordHasher()
	encoded, err := h.Hash("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	ok, err := h.Verify("wrong-password", encoded)
	if err != nil {
		t.Fatalf("verify errored: %v", err)
	}
	if ok {
		t.Fatal("expected verify to reject a wrong password")
	}
}

func TestHashProducesDistinctOutputsForSameInput(t *testing.T) {
	h := NewArgon2idPasswordHasher()
	a, err := h.Hash("same-input")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	b, err := h.Hash("same-input")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if a == b {
		t.Fatal("expected distinct salts to yield distinct encoded hashes")
	}
}

func TestVerifyRejectsMalformedEncoding(t *testing.T) {
	h := NewArgon2idPasswordHasher()
	if _, err := h.Verify("any", "not-a-phc-string"); err == nil {
		t.Fatal("expected error for malformed PHC")
	}
}
