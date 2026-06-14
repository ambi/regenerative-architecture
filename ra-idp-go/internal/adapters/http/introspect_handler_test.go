package http

// SCL シナリオ "有効なトークンの introspection は active=true を返す" / "失効済みトークンは active=false のみ"
// を /introspect 経由で検証する。confidential client の認証が前提。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

const (
	introClientID = "rs-client"
	introSecret   = "rs-client-secret"
)

func newIntrospectServer(intro *fakeIntrospector, denylist *fakeDenylist) *echo.Echo {
	clientRepo := memory.NewClientRepository()
	secretHash := domain.HashClientSecret(introSecret)
	clientRepo.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID,
		ClientID: introClientID, ClientSecretHash: &secretHash,
		ClientType:   spec.ClientConfidential,
		RedirectURIs: []string{"https://rs.example/cb"}, GrantTypes: []spec.GrantType{spec.GrantClientCredentials},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic, Scope: "api",
		FapiProfile: spec.FapiNone, CreatedAt: time.Now().UTC(),
	})
	e := echo.New()
	deps := Deps{
		Issuer:            "http://test",
		ClientRepo:        clientRepo,
		TokenIntrospector: intro,
		RefreshStore:      memory.NewRefreshTokenStore(),
	}
	if denylist != nil {
		deps.AccessTokenDenylist = denylist
	}
	Register(e, deps)
	return e
}

func postIntrospect(t *testing.T, e *echo.Echo, token string) (int, map[string]any) {
	t.Helper()
	form := url.Values{"token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(introClientID, introSecret)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var body map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
	}
	return rec.Code, body
}

func TestIntrospectReturnsActiveTrueWithClaims(t *testing.T) {
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, JTI: "jti-active", ClientID: "demo-client", Sub: "user_alice",
		Scope: "openid profile", TokenType: "access_token",
		Exp: time.Now().Add(time.Hour).Unix(), Iat: time.Now().Unix(),
	}}
	e := newIntrospectServer(intro, nil)
	status, body := postIntrospect(t, e, "atoken")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if body["active"] != true {
		t.Fatalf("active=%v, want true (body=%v)", body["active"], body)
	}
	for _, field := range []string{"sub", "scope", "client_id", "exp", "iat"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("missing required claim %q in active=true response: %v", field, body)
		}
	}
}

func TestIntrospectReturnsActiveFalseAloneForRevokedToken(t *testing.T) {
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, JTI: "jti-revoked", ClientID: "demo-client", Sub: "user_alice",
		Scope: "openid",
	}}
	denylist := &fakeDenylist{revoked: map[string]bool{"jti-revoked": true}}
	e := newIntrospectServer(intro, denylist)
	status, body := postIntrospect(t, e, "atoken")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if body["active"] != false {
		t.Fatalf("active=%v, want false (body=%v)", body["active"], body)
	}
	// RFC 7662 §2.2: failed introspection は active のみを返す。
	for _, field := range []string{"sub", "scope", "client_id", "exp", "iat", "jti"} {
		if v, ok := body[field]; ok {
			t.Fatalf("active=false response must not include %q (got %v)", field, v)
		}
	}
}

func TestIntrospectInactiveTokenReturnsActiveFalseAlone(t *testing.T) {
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{Active: false}}
	e := newIntrospectServer(intro, nil)
	status, body := postIntrospect(t, e, "expired-token")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if body["active"] != false || len(body) != 1 {
		t.Fatalf("expected active=false alone, got %v", body)
	}
}
