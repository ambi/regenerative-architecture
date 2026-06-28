package http_test

// PAR (RFC 9126) のシナリオテスト。
// SCL invariant `ParRequestUriSingleUse` と
// scenario "PAR 経由の request_uri は一度だけ参照できる" を Go テストで担保する。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	httpadapter "ra-idp-go/internal/shared/adapters/http/server"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

const (
	parClientID     = "par-client"
	parClientSecret = "par-client-secret"
	parRedirectURI  = "https://app.example.com/cb"
)

func newPARTestServer(t *testing.T) *echo.Echo {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	secretHash := domain.HashClientSecret(parClientSecret)
	clientRepo.Seed(&spec.OAuth2Client{
		TenantID: spec.DefaultTenantID,
		ClientID: parClientID, ClientSecretHash: &secretHash,
		ClientType:              spec.ClientConfidential,
		RedirectURIs:            []string{parRedirectURI},
		GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic,
		Scope:                   "openid profile",
		FapiProfile:             spec.FapiNone,
		CreatedAt:               time.Now().UTC(),
	})
	e := echo.New()
	httpadapter.Register(e, support.Deps{
		Issuer:       "http://test",
		ClientRepo:   clientRepo,
		PARStore:     memory.NewPARStore(),
		RequestStore: memory.NewAuthorizationRequestStore(),
		CodeStore:    memory.NewAuthorizationCodeStore(),
	})
	return e
}

func postPAR(t *testing.T, e *echo.Echo, form url.Values) (status int, requestURI string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/par", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(parClientID, parClientSecret)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		return rec.Code, ""
	}
	var body struct {
		RequestURI string `json:"request_uri"`
		ExpiresIn  int    `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode PAR body: %v", err)
	}
	return rec.Code, body.RequestURI
}

func getAuthorize(e *echo.Echo, query url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/authorize?"+query.Encode(), http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestPushAuthorizationRequestRoundTripsToAuthorize(t *testing.T) {
	e := newPARTestServer(t)
	// 仕様: code_challenge は S256 必須、redirect_uri は登録済みと完全一致。
	parForm := url.Values{
		"client_id":             {parClientID},
		"redirect_uri":          {parRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid"},
		"code_challenge":        {"abcdef0123456789abcdef0123456789abcdef0123ab"},
		"code_challenge_method": {"S256"},
	}
	status, requestURI := postPAR(t, e, parForm)
	if status != http.StatusCreated {
		t.Fatalf("/par status=%d, want 201", status)
	}
	if !strings.HasPrefix(requestURI, "urn:ietf:params:oauth:request_uri:") {
		t.Fatalf("unexpected request_uri: %q", requestURI)
	}

	// 1 回目の /authorize は受理されて /login へリダイレクト。
	rec := getAuthorize(e, url.Values{"request_uri": {requestURI}, "client_id": {parClientID}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("first /authorize status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 2 回目の /authorize は invalid_request_uri (`ParRequestUriSingleUse`).
	rec = getAuthorize(e, url.Values{"request_uri": {requestURI}, "client_id": {parClientID}})
	if rec.Code == http.StatusSeeOther ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"invalid_request_uri"`)) {
		t.Fatalf("second /authorize: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPushAuthorizationRequestRejectsCrossTenantConsumption(t *testing.T) {
	// PAR record を tenant=acme で保存して、/authorize は default tenant (bare 経路) に
	// 投げる。handleAuthorize は consumed.TenantID != support.RequestTenantID(c) を理由に拒否する。
	store := memory.NewPARStore()
	// 別テナントの PAR レコードを直接 store に保存。
	rec := &spec.PARRecord{
		TenantID:   "acme",
		RequestURI: "urn:ietf:params:oauth:request_uri:cross-tenant",
		ClientID:   parClientID,
		Parameters: map[string]string{
			"client_id":             parClientID,
			"redirect_uri":          parRedirectURI,
			"response_type":         "code",
			"scope":                 "openid",
			"code_challenge":        "abcdef0123456789abcdef0123456789abcdef0123ab",
			"code_challenge_method": "S256",
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("seed PAR: %v", err)
	}
	// PARStore を差し替えた Deps で再 Register。
	e := echo.New()
	clientRepo := memory.NewClientRepository()
	secretHash := domain.HashClientSecret(parClientSecret)
	clientRepo.Seed(&spec.OAuth2Client{
		TenantID: spec.DefaultTenantID,
		ClientID: parClientID, ClientSecretHash: &secretHash,
		ClientType: spec.ClientConfidential, RedirectURIs: []string{parRedirectURI},
		GrantTypes:    []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes: []spec.ResponseType{spec.ResponseTypeCode}, FapiProfile: spec.FapiNone,
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic, Scope: "openid",
		CreatedAt: time.Now().UTC(),
	})
	httpadapter.Register(e, support.Deps{
		Issuer:       "http://test",
		ClientRepo:   clientRepo,
		PARStore:     store,
		RequestStore: memory.NewAuthorizationRequestStore(),
		CodeStore:    memory.NewAuthorizationCodeStore(),
	})
	out := getAuthorize(e, url.Values{"request_uri": {rec.RequestURI}, "client_id": {parClientID}})
	if out.Code == http.StatusSeeOther ||
		!bytes.Contains(out.Body.Bytes(), []byte(`"error":"invalid_request_uri"`)) {
		t.Fatalf("cross-tenant /authorize: status=%d body=%s", out.Code, out.Body.String())
	}
}
