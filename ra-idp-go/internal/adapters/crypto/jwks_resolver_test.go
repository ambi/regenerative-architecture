package crypto

import "testing"

func TestValidateJWKSURI(t *testing.T) {
	for _, valid := range []string{
		"https://client.example/.well-known/jwks.json",
		"https://client.example:8443/keys",
	} {
		if err := ValidateJWKSURI(valid); err != nil {
			t.Errorf("%s rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{
		"http://client.example/keys",
		"https://user:pass@client.example/keys",
		"https://client.example/keys#fragment",
		"/relative",
	} {
		if err := ValidateJWKSURI(invalid); err == nil {
			t.Errorf("%s accepted", invalid)
		}
	}
}

func TestPublicIPCheckRejectsInternalRanges(t *testing.T) {
	resolver := NewJWKResolver()
	for _, host := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "192.168.1.1"} {
		if _, err := resolver.safeIPs(t.Context(), host); err == nil {
			t.Errorf("%s accepted", host)
		}
	}
}
