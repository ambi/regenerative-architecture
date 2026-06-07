package http_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	httpadapter "ra-idp-go/internal/adapters/http"
	"ra-idp-go/internal/adapters/persistence/memory"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
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

	secretHash := domain.HashClientSecret(demoClientSecret)
	clientRepo.Seed(&spec.Client{
		ClientID: demoClientID, ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
		RedirectURIs: []string{demoRedirectURI},
		GrantTypes: []spec.GrantType{
			spec.GrantAuthorizationCode, spec.GrantRefreshToken,
			spec.GrantClientCredentials, spec.GrantDeviceCode,
		},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		Scope:                    "openid profile email offline_access",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                time.Now().UTC(),
	})

	hash, err := hasher.Hash(demoPassword)
	if err != nil {
		t.Fatalf("seed password: %v", err)
	}
	email := "alice@example.com"
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "user_alice", PreferredUsername: demoUsername, PasswordHash: hash,
		Email: &email, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})

	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		t.Fatalf("key store: %v", err)
	}
	tokenIssuer := crypto.NewJWTSigner("http://test", keyStore)
	sessionManager := authusecases.NewSessionManager(memory.NewSessionStore())
	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer:     "http://test",
		ClientRepo: clientRepo, UserRepo: userRepo, ConsentRepo: memory.NewConsentRepository(),
		RequestStore: requestStore, CodeStore: codeStore, PARStore: memory.NewPARStore(),
		RefreshStore: memory.NewRefreshTokenStore(), DeviceCodeStore: memory.NewDeviceCodeStore(),
		KeyStore: keyStore, TokenIssuer: tokenIssuer, TokenIntrospector: tokenIssuer,
		PasswordHasher: hasher, SessionManager: sessionManager, AuthnResolver: sessionManager,
	})
	return httptest.NewServer(e)
}

func TestBrowserAuthorizationFlowUsesCookiesAndJSONAPI(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	client := browserClient(t)

	verifier := "this-is-a-cryptographically-fine-verifier-for-pkce-tests"
	state := "opaque-state"
	resp := startAuthorization(t, client, srv.URL, verifier, state)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("/authorize status=%d, want 303", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/login" {
		t.Fatalf("/authorize Location=%q, want /login", location)
	}
	transactionCookie := findCookie(resp.Cookies(), "ra_idp_transaction")
	if transactionCookie == nil || !transactionCookie.HttpOnly {
		t.Fatal("authorization transaction cookie must be HttpOnly")
	}
	resp.Body.Close()

	transaction := getJSON[struct {
		Kind      string `json:"kind"`
		CSRFToken string `json:"csrf_token"`
	}](t, client, srv.URL+"/api/auth/transaction")
	if transaction.Kind != "login" || transaction.CSRFToken == "" {
		t.Fatalf("unexpected login transaction: %+v", transaction)
	}
	if strings.Contains(mustJSON(t, transaction), transactionCookie.Value) {
		t.Fatal("browser API exposed the internal authorization request ID")
	}

	loginResult := postJSON[map[string]string](t, client, srv.URL+"/api/auth/login", transaction.CSRFToken, map[string]string{
		"username": demoUsername,
		"password": demoPassword,
	})
	if loginResult["next"] != "/consent" {
		t.Fatalf("login next=%q, want /consent", loginResult["next"])
	}

	consent := getJSON[struct {
		Kind       string   `json:"kind"`
		CSRFToken  string   `json:"csrf_token"`
		ClientName string   `json:"client_name"`
		Scopes     []string `json:"scopes"`
	}](t, client, srv.URL+"/api/auth/transaction")
	if consent.Kind != "consent" || consent.ClientName != demoClientID {
		t.Fatalf("unexpected consent transaction: %+v", consent)
	}

	consentResult := postJSON[map[string]string](
		t,
		client,
		srv.URL+"/api/auth/consent",
		consent.CSRFToken,
		map[string]string{"action": "allow"},
	)
	redirectTo, err := url.Parse(consentResult["redirect_to"])
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	code := redirectTo.Query().Get("code")
	if code == "" || redirectTo.Query().Get("state") != state {
		t.Fatalf("invalid authorization redirect: %s", redirectTo)
	}

	tokenForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {demoRedirectURI},
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/token", strings.NewReader(tokenForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(demoClientID, demoClientSecret)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /token: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`"access_token"`)) {
		t.Fatalf("/token status=%d body=%s", resp.StatusCode, body)
	}
}

func TestBrowserAPIPostRejectsMissingCSRF(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	client := browserClient(t)
	resp := startAuthorization(t, client, srv.URL, "verifier-for-csrf-test-12345678901234567890", "state")
	resp.Body.Close()
	_ = getJSON[map[string]any](t, client, srv.URL+"/api/auth/transaction")

	payload, _ := json.Marshal(map[string]string{"username": demoUsername, "password": demoPassword})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://test")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d, want 403; body=%s", resp.StatusCode, body)
	}
}

func TestBrowserAPIPostRejectsForeignOrigin(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	client := browserClient(t)
	resp := startAuthorization(t, client, srv.URL, "verifier-for-origin-test-123456789012345678", "state")
	resp.Body.Close()
	transaction := getJSON[struct {
		CSRFToken string `json:"csrf_token"`
	}](t, client, srv.URL+"/api/auth/transaction")

	payload, _ := json.Marshal(map[string]string{"username": demoUsername, "password": demoPassword})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", transaction.CSRFToken)
	req.Header.Set("Origin", "https://attacker.example")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", resp.StatusCode)
	}
}

func TestGoDoesNotServeFrontendAssets(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	for _, path := range []string{"/login", "/ui/assets/app.css", "/ui/assets/app.js"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s status=%d, want 404", path, resp.StatusCode)
		}
	}
}

func startAuthorization(
	t *testing.T,
	client *http.Client,
	baseURL, verifier, state string,
) *http.Response {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	query := url.Values{
		"client_id":             {demoClientID},
		"redirect_uri":          {demoRedirectURI},
		"response_type":         {"code"},
		"scope":                 {"openid profile email offline_access"},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}
	resp, err := client.Get(baseURL + "/authorize?" + query.Encode())
	if err != nil {
		t.Fatalf("GET /authorize: %v", err)
	}
	return resp
}

func browserClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func getJSON[T any](t *testing.T, client *http.Client, target string) T {
	t.Helper()
	resp, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s status=%d body=%s", target, resp.StatusCode, body)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode %s: %v", target, err)
	}
	return result
}

func postJSON[T any](
	t *testing.T,
	client *http.Client,
	target, csrf string,
	payload any,
) T {
	t.Helper()
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	req.Header.Set("Origin", "http://test")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s status=%d body=%s", target, resp.StatusCode, responseBody)
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode %s: %v", target, err)
	}
	return result
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(body)
}
