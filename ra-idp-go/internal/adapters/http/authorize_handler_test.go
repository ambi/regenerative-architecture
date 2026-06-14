package http

// SCL シナリオ "prompt=none で session 無し" / "prompt=login" / "prompt=consent" /
// "max_age を超えた前回認証では再認証を要求する" を handler 層で検証する。
// AuthnResolver の差し替えだけで再認証フローを観測する単純構成。

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type fakeAuthnResolver struct {
	ctx *authdomain.AuthenticationContext
}

func (f *fakeAuthnResolver) Resolve(_ context.Context, _ authdomain.Headers) (*authdomain.AuthenticationContext, error) {
	return f.ctx, nil
}

const (
	authClientID    = "auth-client"
	authClientSec   = "auth-client-secret"
	authRedirectURI = "https://app.example.com/cb"
)

func newAuthorizeTestServer(t *testing.T, authn *authdomain.AuthenticationContext, consent *spec.Consent) *echo.Echo {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	consentRepo := memory.NewConsentRepository()
	secretHash := domain.HashClientSecret(authClientSec)
	now := time.Now().UTC()
	clientRepo.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID,
		ClientID: authClientID, ClientSecretHash: &secretHash,
		ClientType: spec.ClientConfidential, RedirectURIs: []string{authRedirectURI},
		GrantTypes:               []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		Scope:                    "openid profile",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                now,
	})
	if authn != nil {
		userRepo.Seed(&spec.User{
			Sub: authn.Sub, PreferredUsername: "alice",
			TenantID: spec.DefaultTenantID, CreatedAt: now, UpdatedAt: now,
		})
	}
	if consent != nil {
		_ = consentRepo.Save(context.Background(), consent)
	}
	e := echo.New()
	deps := Deps{
		Issuer:       "http://test",
		ClientRepo:   clientRepo,
		UserRepo:     userRepo,
		ConsentRepo:  consentRepo,
		RequestStore: memory.NewAuthorizationRequestStore(),
		CodeStore:    memory.NewAuthorizationCodeStore(),
		PARStore:     memory.NewPARStore(),
	}
	if authn != nil {
		deps.AuthnResolver = &fakeAuthnResolver{ctx: authn}
	}
	Register(e, deps)
	return e
}

func authorizeQuery(extra url.Values) url.Values {
	q := url.Values{
		"client_id":             {authClientID},
		"redirect_uri":          {authRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid profile"},
		"code_challenge":        {"abcdef0123456789abcdef0123456789abcdef0123ab"},
		"code_challenge_method": {"S256"},
	}
	for k, v := range extra {
		q[k] = v
	}
	return q
}

func runAuthorize(t *testing.T, e *echo.Echo, q url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/authorize?"+q.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAuthorizePromptNoneWithoutSessionReturnsLoginRequired(t *testing.T) {
	e := newAuthorizeTestServer(t, nil, nil)
	rec := runAuthorize(t, e, authorizeQuery(url.Values{"prompt": {"none"}}))
	if rec.Code == http.StatusSeeOther ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"login_required"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizePromptLoginForcesReauthentication(t *testing.T) {
	authn := &authdomain.AuthenticationContext{
		Sub: "user_alice", AuthTime: time.Now().Unix(), AMR: []string{"pwd"},
	}
	e := newAuthorizeTestServer(t, authn, nil)
	rec := runAuthorize(t, e, authorizeQuery(url.Values{"prompt": {"login"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/login") {
		t.Fatalf("redirect Location=%q, want /login", loc)
	}
}

func TestAuthorizeMaxAgeBeyondLastAuthForcesReauthentication(t *testing.T) {
	// auth_time が 1 時間前、max_age=60 → NeedsReauthentication=true。
	authn := &authdomain.AuthenticationContext{
		Sub: "user_alice", AuthTime: time.Now().Add(-time.Hour).Unix(), AMR: []string{"pwd"},
	}
	e := newAuthorizeTestServer(t, authn, nil)
	rec := runAuthorize(t, e, authorizeQuery(url.Values{"max_age": {"60"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/login") {
		t.Fatalf("redirect Location=%q, want /login", loc)
	}
}

func TestAuthorizePromptConsentBypassesExistingConsent(t *testing.T) {
	now := time.Now().UTC()
	authn := &authdomain.AuthenticationContext{
		Sub: "user_alice", AuthTime: now.Unix(), AMR: []string{"pwd"},
	}
	// 既存 Consent。prompt=consent が無ければ即 issueCode に進む。
	consent := &spec.Consent{
		TenantID: spec.DefaultTenantID,
		Sub:      "user_alice", ClientID: authClientID,
		Scopes:    []string{"openid", "profile"},
		State:     spec.ConsentGranted,
		GrantedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	e := newAuthorizeTestServer(t, authn, consent)
	rec := runAuthorize(t, e, authorizeQuery(url.Values{"prompt": {"consent"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "/consent") {
		t.Fatalf("redirect Location=%q, want /consent", loc)
	}
}
