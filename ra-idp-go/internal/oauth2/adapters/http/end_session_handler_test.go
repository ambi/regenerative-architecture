package http_test

// SCL シナリオ "RP-Initiated Logout は登録済み post_logout_redirect_uri にだけリダイレクトする"
// と "未登録 post_logout_redirect_uri は拒否される" を /end_session 経由で検証する。

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	httpadapter "ra-idp-go/internal/infrastructure/http"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

const (
	logoutClientID    = "logout-client"
	logoutRedirectURI = "https://app.example.com/post-logout"
)

func newEndSessionServer(t *testing.T) *echo.Echo {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	clientRepo.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID,
		ClientID: logoutClientID, ClientType: spec.ClientPublic,
		RedirectURIs:             []string{logoutRedirectURI},
		GrantTypes:               []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodNone,
		Scope:                    "openid",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                time.Now().UTC(),
	})
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:     "http://test",
		ClientRepo: clientRepo,
	})
	return e
}

func TestEndSessionRedirectsToRegisteredURIWithStatePropagation(t *testing.T) {
	e := newEndSessionServer(t)
	q := url.Values{
		"client_id":                {logoutClientID},
		"post_logout_redirect_uri": {logoutRedirectURI},
		"state":                    {"opaque-state-123"},
	}
	req := httptest.NewRequest(http.MethodGet, "/end_session?"+q.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if loc.Scheme+"://"+loc.Host+loc.Path != logoutRedirectURI {
		t.Fatalf("redirect URI mismatch: got %s want %s", loc, logoutRedirectURI)
	}
	if got := loc.Query().Get("state"); got != "opaque-state-123" {
		t.Fatalf("state propagation: got %q, want %q", got, "opaque-state-123")
	}
}

func TestEndSessionRejectsUnregisteredPostLogoutURI(t *testing.T) {
	e := newEndSessionServer(t)
	q := url.Values{
		"client_id":                {logoutClientID},
		"post_logout_redirect_uri": {"https://evil.example.com/cb"},
	}
	req := httptest.NewRequest(http.MethodGet, "/end_session?"+q.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusFound ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"invalid_request"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
