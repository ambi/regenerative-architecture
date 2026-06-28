package http_test

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

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/oauth2/domain"
	httpadapter "ra-idp-go/internal/shared/adapters/http/server"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

type fakeAuthnResolver struct {
	ctx *authdomain.AuthenticationContext
}

func (f *fakeAuthnResolver) Resolve(_ context.Context, _ authdomain.Headers) (*authdomain.AuthenticationContext, error) {
	return f.ctx, nil
}

const (
	authClientID           = "auth-client"
	authClientSec          = "auth-client-secret"
	authRedirectURI        = "https://app.example.com/cb"
	authFirstPartyClientID = "auth-client-fp"
)

func newAuthorizeTestServer(t *testing.T, authn *authdomain.AuthenticationContext, consent *spec.Consent) *echo.Echo {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	consentRepo := memory.NewConsentRepository()
	secretHash := domain.HashClientSecret(authClientSec)
	now := time.Now().UTC()
	clientRepo.Seed(&spec.OAuth2Client{
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
	// first-party クライアント (ADR-061): consent をスキップする検証用。
	clientRepo.Seed(&spec.OAuth2Client{
		TenantID: spec.DefaultTenantID,
		ClientID: authFirstPartyClientID, ClientType: spec.ClientPublic,
		RedirectURIs:             []string{authRedirectURI},
		GrantTypes:               []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodNone,
		Scope:                    "openid profile ra.admin",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		FirstParty:               true,
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
	deps := support.Deps{
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
	httpadapter.Register(e, deps)
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

// ADR-061: first-party クライアントは consent 画面をスキップし、同意レコードが
// 無くても即 authorization code を発行する (redirect_uri へ code 付きで 303)。
func TestAuthorizeFirstPartyClientSkipsConsent(t *testing.T) {
	now := time.Now().UTC()
	authn := &authdomain.AuthenticationContext{
		Sub: "user_alice", AuthTime: now.Unix(), AMR: []string{"pwd"},
	}
	e := newAuthorizeTestServer(t, authn, nil)
	q := authorizeQuery(url.Values{})
	q.Set("client_id", authFirstPartyClientID)
	q.Set("scope", "openid profile ra.admin")
	rec := runAuthorize(t, e, q)
	// 認可コード発行は redirect_uri へ 302 (Found)。/consent への 303 ではない。
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if strings.Contains(loc, "/consent") {
		t.Fatalf("first-party client must skip consent, got Location=%q", loc)
	}
	if !strings.HasPrefix(loc, authRedirectURI) || !strings.Contains(loc, "code=") {
		t.Fatalf("expected redirect to %s with code, got Location=%q", authRedirectURI, loc)
	}
}
