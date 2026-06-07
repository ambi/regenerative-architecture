package usecases

import (
	"regexp"
	"testing"
)

const rfc6238SHA1SecretBase32 = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestGenerateTOTPRFC6238Vectors(t *testing.T) {
	for _, tc := range []struct {
		at   int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		got, err := GenerateTOTP(rfc6238SHA1SecretBase32, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("T=%d: got %s, want %s", tc.at, got, tc.want)
		}
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	now := int64(1_700_000_000)
	previous, err := GenerateTOTP(rfc6238SHA1SecretBase32, now-30)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyTOTP(rfc6238SHA1SecretBase32, previous, now, 1) {
		t.Fatal("previous time step should be accepted")
	}
	if VerifyTOTP(rfc6238SHA1SecretBase32, previous, now+30, 1) {
		t.Fatal("code two time steps away should be rejected")
	}
}

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[A-Z2-7]{32}$`).MatchString(secret) {
		t.Fatalf("unexpected secret %q", secret)
	}
}

func TestBuildOTPAuthURI(t *testing.T) {
	uri := BuildOTPAuthURI(rfc6238SHA1SecretBase32, "alice@example.com", "RA IdP")
	for _, part := range []string{
		"otpauth://totp/",
		"secret=" + rfc6238SHA1SecretBase32,
		"issuer=RA+IdP",
		"algorithm=SHA1",
		"digits=6",
		"period=30",
	} {
		if !regexp.MustCompile(regexp.QuoteMeta(part)).MatchString(uri) {
			t.Fatalf("URI %q does not contain %q", uri, part)
		}
	}
}
