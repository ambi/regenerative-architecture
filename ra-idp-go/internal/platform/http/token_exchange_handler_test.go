package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/platform/crypto"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

const (
	exchClientID     = "exch-client"
	exchClientSecret = "exch-client-secret"
)

// newTokenExchangeServer は client_credentials と token-exchange を許可した
// confidential client を持つ最小サーバを返す。subject_token は client_credentials で発行する。
func newTokenExchangeServer(t *testing.T) string {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	secretHash := domain.HashClientSecret(exchClientSecret)
	clientRepo.Seed(&spec.Client{
		ClientID: exchClientID, ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
		GrantTypes: []spec.GrantType{
			spec.GrantClientCredentials, spec.GrantTokenExchange,
		},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic,
		Scope:                   "read write",
		FapiProfile:             spec.FapiNone,
		CreatedAt:               time.Now().UTC(),
	})
	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		t.Fatalf("key store: %v", err)
	}
	tokenIssuer := crypto.NewJWTSigner("http://test", keyStore)
	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer:       "http://test",
		ClientRepo:   clientRepo,
		UserRepo:     memory.NewUserRepository(),
		ConsentRepo:  memory.NewConsentRepository(),
		RefreshStore: memory.NewRefreshTokenStore(),
		KeyStore:     keyStore, TokenIssuer: tokenIssuer, TokenIntrospector: tokenIssuer,
	})
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv.URL
}

func postToken(t *testing.T, base string, form url.Values) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(exchClientID, exchClientSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	_ = json.Unmarshal(body, &parsed)
	return resp.StatusCode, parsed
}

func mintSubjectToken(t *testing.T, base string) string {
	t.Helper()
	status, body := postToken(t, base, url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {"read write"},
	})
	if status != http.StatusOK {
		t.Fatalf("client_credentials status=%d body=%v", status, body)
	}
	tok, _ := body["access_token"].(string)
	if tok == "" {
		t.Fatalf("no access_token in %v", body)
	}
	return tok
}

func TestTokenExchangeIssuesDelegatedToken(t *testing.T) {
	base := newTokenExchangeServer(t)
	subject := mintSubjectToken(t, base)

	resource := "https://api.example/orders"
	status, body := postToken(t, base, url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {subject},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"resource":           {resource},
	})
	if status != http.StatusOK {
		t.Fatalf("token-exchange status=%d body=%v", status, body)
	}
	if got := body["issued_token_type"]; got != "urn:ietf:params:oauth:token-type:access_token" {
		t.Fatalf("issued_token_type=%v", got)
	}
	issued, _ := body["access_token"].(string)
	if issued == "" {
		t.Fatalf("no access_token in %v", body)
	}

	// 発行トークンを introspect して aud / act を検証する。
	intReq, _ := http.NewRequest(http.MethodPost, base+"/introspect",
		strings.NewReader(url.Values{"token": {issued}}.Encode()))
	intReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	intReq.SetBasicAuth(exchClientID, exchClientSecret)
	intResp, err := http.DefaultClient.Do(intReq)
	if err != nil {
		t.Fatalf("POST /introspect: %v", err)
	}
	defer intResp.Body.Close()
	var introspection map[string]any
	intBody, _ := io.ReadAll(intResp.Body)
	if err := json.Unmarshal(intBody, &introspection); err != nil {
		t.Fatalf("introspect json: %v", err)
	}
	if active, _ := introspection["active"].(bool); !active {
		t.Fatalf("issued token not active: %v", introspection)
	}
	aud, _ := introspection["aud"].([]any)
	if len(aud) != 1 || aud[0] != resource {
		t.Fatalf("aud=%v, want [%s]", introspection["aud"], resource)
	}
	act, _ := introspection["act"].(map[string]any)
	if act == nil || act["sub"] != exchClientID {
		t.Fatalf("act=%v, want act.sub=%s", introspection["act"], exchClientID)
	}
}

func TestTokenExchangeRejectsInvalidSubjectToken(t *testing.T) {
	base := newTokenExchangeServer(t)
	status, body := postToken(t, base, url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token": {"not-a-real-token"},
		"resource":      {"https://api.example/orders"},
	})
	if status == http.StatusOK {
		t.Fatalf("invalid subject_token was accepted: %v", body)
	}
}

func TestTokenExchangeRejectsMissingResource(t *testing.T) {
	base := newTokenExchangeServer(t)
	subject := mintSubjectToken(t, base)
	status, body := postToken(t, base, url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token": {subject},
	})
	if status == http.StatusOK {
		t.Fatalf("missing resource was accepted: %v", body)
	}
}
