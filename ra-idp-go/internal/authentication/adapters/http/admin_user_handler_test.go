package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/platform/crypto"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func TestDisabledUserCannotLogIn(t *testing.T) {
	repo := memory.NewUserRepository()
	requestStore := memory.NewAuthorizationRequestStore()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("current-password-1")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	repo.Seed(&spec.User{
		Sub: "disabled", PreferredUsername: "disabled", PasswordHash: hash,
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusDisabled, StatusChangedAt: &now},
		CreatedAt: now, UpdatedAt: now,
	})
	if err := requestStore.Save(context.Background(), &spec.AuthorizationRequest{
		ID: "transaction", State: spec.AuthFlowReceived, ExpiresAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer: "http://idp.test", UserRepo: repo, RequestStore: requestStore,
		PasswordHasher: hasher,
	})
	csrf, csrfCookie := passwordResetCSRF(t, e)
	requestBody, _ := json.Marshal(map[string]string{
		"username": "disabled", "password": "current-password-1",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://idp.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(csrfCookie)
	request.AddCookie(&http.Cookie{Name: "ra_idp_transaction", Value: "transaction"})
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"invalid_credentials"`) {
		t.Fatalf("unexpected body=%s", response.Body.String())
	}
}

func adminCSRF(t *testing.T, e *echo.Echo) (string, *http.Cookie) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/account", http.NoBody)
	request.Header.Set("X-Demo-Sub", "admin")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("account status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("csrf cookie missing")
	}
	if cookies[0].Path != "/" {
		t.Fatalf("csrf cookie path=%q, want /", cookies[0].Path)
	}
	return body.CSRFToken, cookies[0]
}

func adminJSONRequest(
	t *testing.T,
	e *echo.Echo,
	method, path, csrf string,
	cookie *http.Cookie,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://idp.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.Header.Set("X-Demo-Sub", "admin")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	return response
}
