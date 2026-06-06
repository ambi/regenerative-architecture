package http_test

// /authorize → /login → /token を in-process Echo で結合する end-to-end テスト。

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"

	httpadapter "ra-idp-go/internal/adapters/http"

	authusecases "ra-idp-go/internal/authentication/usecases"
)

const (
	demoClientID     = "demo-client"
	demoClientSecret = "demo-client-secret"
	demoRedirectURI  = "http://localhost:3000/callback"
	demoUsername     = "alice"
	demoPassword     = "demo-password-1234"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()

	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	requestStore := memory.NewAuthorizationRequestStore()
	codeStore := memory.NewAuthorizationCodeStore()
	hasher := crypto.NewArgon2idPasswordHasher()

	secret := demoClientSecret
	secretHash := domain.HashClientSecret(secret)
	clientRepo.Seed(&spec.Client{
		ClientID:                 demoClientID,
		ClientSecretHash:         &secretHash,
		ClientType:               spec.ClientConfidential,
		RedirectURIs:             []string{demoRedirectURI},
		GrantTypes:               []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		Scope:                    "openid profile email",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                time.Now().UTC(),
	})

	hash, err := hasher.Hash(demoPassword)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	email := "alice@example.com"
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub:               "user_alice",
		PreferredUsername: demoUsername,
		PasswordHash:      hash,
		Email:             &email,
		EmailVerified:     true,
		CreatedAt:         now,
		UpdatedAt:         now,
	})

	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		t.Fatalf("key store: %v", err)
	}
	tokenIssuer := crypto.NewJWTSigner("http://test", keyStore)
	sessionStore := memory.NewSessionStore()
	sessionMgr := authusecases.NewSessionManager(sessionStore)
	refreshStore := memory.NewRefreshTokenStore()
	consentRepo := memory.NewConsentRepository()
	// e2e テストはコンセント済みクライアント前提で /login → 302 を期待する。
	_ = consentRepo.Save(context.Background(), &spec.Consent{
		Sub: "user_alice", ClientID: demoClientID,
		Scopes: []string{"openid", "profile", "email"}, State: spec.ConsentGranted,
		GrantedAt: now, ExpiresAt: now.Add(365 * 24 * time.Hour),
	})

	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer:            "http://test",
		ClientRepo:        clientRepo,
		UserRepo:          userRepo,
		ConsentRepo:       consentRepo,
		RequestStore:      requestStore,
		CodeStore:         codeStore,
		RefreshStore:      refreshStore,
		KeyStore:          keyStore,
		TokenIssuer:       tokenIssuer,
		TokenIntrospector: tokenIssuer,
		PasswordHasher:    hasher,
		SessionManager:    sessionMgr,
		AuthnResolver:     sessionMgr,
	})

	return httptest.NewServer(e)
}

// pkceS256 は code_verifier と一致する code_challenge を返す。
func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func TestAuthorizationCodeFlowHappyPath(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	verifier := "this-is-a-cryptographically-fine-verifier-for-pkce-tests"
	challenge := pkceS256(verifier)

	// (1) /authorize → 401 + React UI shell + request_id
	q := url.Values{
		"client_id":             {demoClientID},
		"redirect_uri":          {demoRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid"},
		"state":                 {"opaque-state-xyz"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	resp, err := http.Get(srv.URL + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("GET /authorize: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/authorize: status=%d, want 401", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `/ui/assets/app.js`) {
		t.Fatalf("login page is missing UI bundle: %s", body)
	}
	if resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("login page is missing Content-Security-Policy")
	}
	requestID := extractRequestID(string(body))
	if requestID == "" {
		t.Fatalf("could not extract request_id from login page:\n%s", body)
	}

	// (2) /login POST → 302 redirect with code
	loginClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	form := url.Values{
		"request_id": {requestID},
		"username":   {demoUsername},
		"password":   {demoPassword},
	}
	resp, err = loginClient.PostForm(srv.URL+"/login", form)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("/login: status=%d, want 302; body=%s", resp.StatusCode, body)
	}
	location, err := resp.Location()
	if err != nil {
		t.Fatalf("login response missing Location: %v", err)
	}
	resp.Body.Close()
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("redirect missing code: %s", location)
	}
	if location.Query().Get("state") != "opaque-state-xyz" {
		t.Fatalf("redirect state mismatch: %s", location.Query().Get("state"))
	}

	// (3) /token POST → access_token + id_token
	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {demoRedirectURI},
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/token", strings.NewReader(tokenForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(demoClientID, demoClientSecret)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/token: status=%d, body=%s", resp.StatusCode, body)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode token response: %v; body=%s", err, body)
	}
	if out.AccessToken == "" || out.IDToken == "" {
		t.Fatalf("missing tokens in response: %s", body)
	}
	if out.TokenType != "Bearer" {
		t.Fatalf("token_type=%q, want Bearer", out.TokenType)
	}

	// (4) 二度目の交換は失敗する（code 再利用不可）
	resp, err = http.DefaultClient.Do(mustReclone(t, srv.URL, tokenForm))
	if err != nil {
		t.Fatalf("POST /token (replay): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected replay rejection, got 200: %s", body)
	}
}

func TestCompletedLoginFormCannotBeReused(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	verifier := "verifier-for-stale-login-form-reuse-test-123456789"
	q := url.Values{
		"client_id":             {demoClientID},
		"redirect_uri":          {demoRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid"},
		"state":                 {"stale-form-state"},
		"code_challenge":        {pkceS256(verifier)},
		"code_challenge_method": {"S256"},
	}
	resp, err := http.Get(srv.URL + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("GET /authorize: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	requestID := extractRequestID(string(body))
	if requestID == "" {
		t.Fatal("login page did not contain request ID")
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	form := url.Values{
		"request_id": {requestID},
		"username":   {demoUsername},
		"password":   {demoPassword},
	}
	resp, err = client.PostForm(srv.URL+"/login", form)
	if err != nil {
		t.Fatalf("first POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("first POST /login: status=%d, want 302", resp.StatusCode)
	}

	resp, err = client.PostForm(srv.URL+"/login", form)
	if err != nil {
		t.Fatalf("reused POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("reused POST /login: status=%d, want 302", resp.StatusCode)
	}
	location, err := resp.Location()
	if err != nil {
		t.Fatalf("reused POST /login missing Location: %v", err)
	}
	if location.Query().Get("error") != "invalid_request" {
		t.Fatalf("reused POST /login: error=%q, want invalid_request", location.Query().Get("error"))
	}
	if location.Query().Get("state") != "stale-form-state" {
		t.Fatalf("reused POST /login: state=%q, want stale-form-state", location.Query().Get("state"))
	}
}

func TestAuthorizeRejectsUnknownClient(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	verifier := "another-pkce-verifier-value-1234567890"
	q := url.Values{
		"client_id":             {"nope"},
		"redirect_uri":          {demoRedirectURI},
		"response_type":         {"code"},
		"code_challenge":        {pkceS256(verifier)},
		"code_challenge_method": {"S256"},
	}
	resp, err := http.Get(srv.URL + "/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("GET /authorize: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", resp.StatusCode)
	}
}

func TestUIAssetsAreServed(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	for _, path := range []string{"/ui/assets/app.css", "/ui/assets/app.js"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		if resp.StatusCode != http.StatusOK || len(body) == 0 {
			t.Fatalf("GET %s: status=%d bytes=%d", path, resp.StatusCode, len(body))
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("GET %s: missing nosniff header", path)
		}
	}
}

func TestTokenRejectsWrongPKCE(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	verifier := "verifier-for-the-pkce-mismatch-test"
	challenge := pkceS256(verifier)

	q := url.Values{
		"client_id":             {demoClientID},
		"redirect_uri":          {demoRedirectURI},
		"response_type":         {"code"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	resp, _ := http.Get(srv.URL + "/authorize?" + q.Encode())
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	requestID := extractRequestID(string(body))

	loginClient := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	form := url.Values{
		"request_id": {requestID},
		"username":   {demoUsername},
		"password":   {demoPassword},
	}
	resp, _ = loginClient.PostForm(srv.URL+"/login", form)
	location, _ := resp.Location()
	resp.Body.Close()
	code := location.Query().Get("code")

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {"WRONG-verifier-value"},
		"redirect_uri":  {demoRedirectURI},
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/token", strings.NewReader(tokenForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(demoClientID, demoClientSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected PKCE mismatch rejection, got 200: %s", body)
	}
}

func extractRequestID(html string) string {
	const marker = `"requestId":"`
	_, rest, ok := strings.Cut(html, marker)
	if !ok {
		return ""
	}
	id, _, ok := strings.Cut(rest, `"`)
	if !ok {
		return ""
	}
	return id
}

func mustReclone(t *testing.T, baseURL string, form url.Values) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/token", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("clone token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(demoClientID, demoClientSecret)
	return req
}
