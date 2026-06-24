package domain

import (
	"testing"

	"ra-idp-go/internal/spec"
)

func sampleRP() spec.WsFedRelyingParty {
	return spec.WsFedRelyingParty{
		TenantID:  "default",
		Wtrealm:   "urn:federation:MicrosoftOnline",
		ReplyURLs: []string{"https://login.microsoftonline.com/login.srf", "https://account.activedirectory.windowsazure.com/"},
	}
}

func getter(params map[string]string) func(string) string {
	return func(k string) string { return params[k] }
}

func TestParseSignInRequest(t *testing.T) {
	req := ParseSignInRequest(getter(map[string]string{
		"wa": " wsignin1.0 ", "wtrealm": "urn:federation:MicrosoftOnline",
		"wreply": "https://login.microsoftonline.com/login.srf", "wctx": "id=42",
	}))
	if !req.IsSignIn() {
		t.Fatalf("expected sign-in, got wa=%q", req.Wa)
	}
	if req.Wctx != "id=42" {
		t.Fatalf("wctx = %q, want preserved verbatim", req.Wctx)
	}
}

func TestValidateSignIn_HappyPathWithWreply(t *testing.T) {
	req := ParseSignInRequest(getter(map[string]string{
		"wa": WaSignIn, "wtrealm": "urn:federation:MicrosoftOnline",
		"wreply": "https://account.activedirectory.windowsazure.com/", "wctx": "ctx",
	}))
	got, err := ValidateSignIn(req, sampleRP())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ReplyURL != "https://account.activedirectory.windowsazure.com/" {
		t.Fatalf("ReplyURL = %q", got.ReplyURL)
	}
	if got.Wctx != "ctx" {
		t.Fatalf("Wctx = %q", got.Wctx)
	}
}

func TestValidateSignIn_DefaultsReplyWhenOmitted(t *testing.T) {
	req := ParseSignInRequest(getter(map[string]string{"wa": WaSignIn, "wtrealm": "urn:federation:MicrosoftOnline"}))
	got, err := ValidateSignIn(req, sampleRP())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ReplyURL != "https://login.microsoftonline.com/login.srf" {
		t.Fatalf("expected first reply URL as default, got %q", got.ReplyURL)
	}
}

func TestValidateSignIn_Rejections(t *testing.T) {
	rp := sampleRP()
	tests := map[string]map[string]string{
		"unsupported wa":   {"wa": "wsignout1.0", "wtrealm": rp.Wtrealm},
		"missing wtrealm":  {"wa": WaSignIn},
		"wtrealm mismatch": {"wa": WaSignIn, "wtrealm": "urn:evil"},
		"wreply not allowed (open redirect)": {
			"wa": WaSignIn, "wtrealm": rp.Wtrealm, "wreply": "https://evil.example/steal",
		},
	}
	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			req := ParseSignInRequest(getter(params))
			if _, err := ValidateSignIn(req, rp); err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
		})
	}
}

func TestValidateSignIn_RejectsRPWithoutReplyURLs(t *testing.T) {
	rp := sampleRP()
	rp.ReplyURLs = nil
	req := ParseSignInRequest(getter(map[string]string{"wa": WaSignIn, "wtrealm": rp.Wtrealm}))
	if _, err := ValidateSignIn(req, rp); err == nil {
		t.Fatal("expected error for RP without reply URLs")
	}
}
