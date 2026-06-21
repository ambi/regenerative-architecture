package http_test

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

	httpadapter "ra-idp-go/internal/adapters/http"
	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/platform/crypto"
	"ra-idp-go/internal/platform/notification"
	"ra-idp-go/internal/platform/policy"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func TestPasswordResetHTTPFlow(t *testing.T) {
	e, userRepo, sender, hasher := newPasswordResetHandler(t)
	csrf, cookie := passwordResetCSRF(t, e)

	forgot := serveJSON(t, e, "/api/auth/forgot_password", csrf, cookie, map[string]string{
		"email": "alice@example.com",
	})
	if forgot.Code != http.StatusNoContent {
		t.Fatalf("forgot status=%d body=%s", forgot.Code, forgot.Body.String())
	}
	if len(sender.Sent) != 1 {
		t.Fatalf("sent emails=%d, want 1", len(sender.Sent))
	}
	token := resetTokenFromEmail(t, sender.Sent[0].Text)

	reset := serveJSON(t, e, "/api/auth/reset_password", csrf, cookie, map[string]string{
		"token": token, "new_password": "fresh-password-9182",
	})
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}
	user, err := userRepo.FindBySub(context.Background(), "user-alice")
	if err != nil {
		t.Fatal(err)
	}
	matched, err := hasher.Verify("fresh-password-9182", user.PasswordHash)
	if err != nil || !matched {
		t.Fatalf("new password matched=%v err=%v", matched, err)
	}

	replay := serveJSON(t, e, "/api/auth/reset_password", csrf, cookie, map[string]string{
		"token": token, "new_password": "another-password-9182",
	})
	if replay.Code != http.StatusGone {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
}

func TestForgotPasswordHTTPDoesNotRevealUnknownEmail(t *testing.T) {
	e, _, sender, _ := newPasswordResetHandler(t)
	csrf, cookie := passwordResetCSRF(t, e)
	response := serveJSON(t, e, "/api/auth/forgot_password", csrf, cookie, map[string]string{
		"email": "unknown@example.com",
	})
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(sender.Sent) != 0 {
		t.Fatalf("sent emails=%d, want 0", len(sender.Sent))
	}
}

func newPasswordResetHandler(
	t *testing.T,
) (*echo.Echo, *memory.UserRepository, *notification.NoopEmailSender, *crypto.Argon2idPasswordHasher) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	historyRepo := memory.NewPasswordHistoryRepository()
	tokenStore := memory.NewPasswordResetTokenStore()
	sender := &notification.NoopEmailSender{}
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash("current-password-1")
	if err != nil {
		t.Fatal(err)
	}
	email := "alice@example.com"
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "user-alice", PreferredUsername: "alice", PasswordHash: hash,
		Email: &email, EmailVerified: true, CreatedAt: now, UpdatedAt: now,
	})
	if err := historyRepo.Add(context.Background(), "user-alice", hash, now); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	httpadapter.Register(e, httpadapter.Deps{
		Issuer: "http://idp.test", UserRepo: userRepo, PasswordHasher: hasher,
		PasswordHistoryRepo: historyRepo, PasswordResetTokenStore: tokenStore,
		EmailSender: sender, BreachedPasswordChecker: policy.NoopBreachedPasswordChecker{},
	})
	return e, userRepo, sender, hasher
}

func passwordResetCSRF(t *testing.T, e *echo.Echo) (string, *http.Cookie) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/password_reset_context", http.NoBody)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("context status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	result := response.Result()
	defer result.Body.Close()
	cookies := result.Cookies()
	if len(cookies) != 1 || body.CSRFToken == "" {
		t.Fatalf("csrf=%q cookies=%v", body.CSRFToken, cookies)
	}
	return body.CSRFToken, cookies[0]
}

func serveJSON(
	t *testing.T,
	e *echo.Echo,
	path, csrf string,
	cookie *http.Cookie,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://idp.test")
	request.Header.Set("X-CSRF-Token", csrf)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	return response
}

func resetTokenFromEmail(t *testing.T, message string) string {
	t.Helper()
	start := strings.Index(message, "http://")
	if start < 0 {
		t.Fatalf("reset URL missing from email: %q", message)
	}
	end := strings.IndexByte(message[start:], '\n')
	rawURL := message[start:]
	if end >= 0 {
		rawURL = message[start : start+end]
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatal("reset token missing")
	}
	return token
}
