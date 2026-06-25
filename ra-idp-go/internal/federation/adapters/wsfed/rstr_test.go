package wsfed

import (
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
)

func dummyAssertion() *etree.Element {
	a := etree.NewElement("Assertion")
	a.CreateAttr("xmlns", "urn:oasis:names:tc:SAML:2.0:assertion")
	a.CreateAttr("ID", "_abc")
	a.CreateElement("Issuer").SetText("https://idp.example.com")
	return a
}

func TestBuildRSTR(t *testing.T) {
	created := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	expires := created.Add(5 * time.Minute)

	rstr, err := BuildRSTR(dummyAssertion(), "urn:federation:MicrosoftOnline", "urn:oasis:names:tc:SAML:2.0:assertion", created, expires)
	if err != nil {
		t.Fatalf("BuildRSTR: %v", err)
	}

	if got := rstr.FindElement("//wsa:Address"); got == nil || got.Text() != "urn:federation:MicrosoftOnline" {
		t.Fatalf("AppliesTo address missing or wrong: %+v", got)
	}
	if got := rstr.FindElement("//t:RequestedSecurityToken/Assertion"); got == nil {
		t.Fatal("RequestedSecurityToken does not wrap the assertion")
	}
	if got := rstr.FindElement("//wsu:Created"); got == nil || got.Text() != "2026-06-25T12:00:00Z" {
		t.Fatalf("Lifetime Created missing or wrong: %+v", got)
	}
	if got := rstr.FindElement("//t:TokenType"); got == nil || !strings.Contains(got.Text(), "SAML:2.0:assertion") {
		t.Fatalf("TokenType missing or wrong: %+v", got)
	}
}

func TestBuildRSTR_Rejections(t *testing.T) {
	now := time.Now()
	if _, err := BuildRSTR(nil, "urn:rp", "", now, now.Add(time.Minute)); err == nil {
		t.Fatal("expected error for nil assertion")
	}
	if _, err := BuildRSTR(dummyAssertion(), "  ", "", now, now.Add(time.Minute)); err == nil {
		t.Fatal("expected error for empty appliesTo")
	}
}

func TestRenderPassiveForm(t *testing.T) {
	html, err := RenderPassiveForm("https://login.microsoftonline.com/login.srf", "<RSTR/>", "ctx-42")
	if err != nil {
		t.Fatalf("RenderPassiveForm: %v", err)
	}
	s := string(html)
	if !strings.Contains(s, `action="https://login.microsoftonline.com/login.srf"`) {
		t.Fatalf("form action missing: %s", s)
	}
	if !strings.Contains(s, `name="wa" value="wsignin1.0"`) {
		t.Fatal("wa hidden input missing")
	}
	if !strings.Contains(s, `name="wctx"`) || !strings.Contains(s, "ctx-42") {
		t.Fatal("wctx hidden input missing")
	}
}

func TestRenderPassiveForm_EscapesInjection(t *testing.T) {
	// wctx に細工しても生の </script> 等が属性外へ漏れないこと (html/template が属性エスケープ)。
	html, err := RenderPassiveForm("https://rp.example/acs", "<RSTR/>", `"><script>alert(1)</script>`)
	if err != nil {
		t.Fatalf("RenderPassiveForm: %v", err)
	}
	if strings.Contains(string(html), "<script>alert(1)</script>") {
		t.Fatalf("unescaped script injected via wctx: %s", html)
	}
}

func TestRenderPassiveForm_OmitsEmptyWctx(t *testing.T) {
	html, err := RenderPassiveForm("https://rp.example/acs", "<RSTR/>", "")
	if err != nil {
		t.Fatalf("RenderPassiveForm: %v", err)
	}
	if strings.Contains(string(html), `name="wctx"`) {
		t.Fatal("wctx input should be omitted when empty")
	}
}
