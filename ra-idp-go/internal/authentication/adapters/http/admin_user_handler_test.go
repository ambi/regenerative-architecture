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

	"ra-idp-go/internal/shared/adapters/crypto"
	httpadapter "ra-idp-go/internal/shared/adapters/http/server"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/adapters/persistence/memory"
	"ra-idp-go/internal/shared/spec"

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
	httpadapter.Register(e, support.Deps{
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
