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

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/crypto"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func TestAdminUserAPIRequiresAdminRole(t *testing.T) {
	e, _ := newAdminUserHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	request.Header.Set("X-Demo-Sub", "regular")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAdminUserAPIIsAvailableUnderAPIPath(t *testing.T) {
	e, _ := newAdminUserHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	request.Header.Set("X-Demo-Sub", "admin")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAdminUserAPICreatesAndDisablesUser(t *testing.T) {
	e, repo := newAdminUserHandler(t)
	csrf, cookie := adminCSRF(t, e)

	create := adminJSONRequest(t, e, http.MethodPost, "/api/admin/users", csrf, cookie, map[string]any{
		"preferred_username": "bob",
		"password":           "initial-password-9182",
		"email":              "bob@example.com",
		"roles":              []string{"support"},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "password") {
		t.Fatalf("password material leaked in response: %s", create.Body.String())
	}
	var created struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Sub == "" {
		t.Fatal("created sub is empty")
	}

	disable := adminJSONRequest(
		t, e, http.MethodPost, "/api/admin/users/"+created.Sub+"/disable", csrf, cookie, nil,
	)
	if disable.Code != http.StatusNoContent {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
	user, err := repo.FindBySub(context.Background(), created.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || user.Lifecycle.Status != spec.UserStatusDisabled {
		t.Fatalf("disabled user=%+v", user)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/auth/account", http.NoBody)
	request.Header.Set("X-Demo-Sub", created.Sub)
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled session status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAdminUserAPISetsAndClearsRequiredAction(t *testing.T) {
	e, repo := newAdminUserHandler(t)
	csrf, cookie := adminCSRF(t, e)

	create := adminJSONRequest(t, e, http.MethodPost, "/api/admin/users", csrf, cookie, map[string]any{
		"preferred_username": "carol",
		"password":           "initial-password-9182",
		"email":              "carol@example.com",
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	set := adminJSONRequest(t, e, http.MethodPost,
		"/api/admin/users/"+created.Sub+"/required_actions", csrf, cookie,
		map[string]any{"action": "update_password"})
	if set.Code != http.StatusOK {
		t.Fatalf("set status=%d body=%s", set.Code, set.Body.String())
	}
	var afterSet struct {
		RequiredActions []string `json:"required_actions"`
	}
	if err := json.Unmarshal(set.Body.Bytes(), &afterSet); err != nil {
		t.Fatal(err)
	}
	if len(afterSet.RequiredActions) != 1 || afterSet.RequiredActions[0] != "update_password" {
		t.Fatalf("required_actions=%v, want [update_password]", afterSet.RequiredActions)
	}
	user, err := repo.FindBySub(context.Background(), created.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || len(user.Lifecycle.RequiredActions) != 1 {
		t.Fatalf("persisted required actions=%+v", user)
	}

	bad := adminJSONRequest(t, e, http.MethodPost,
		"/api/admin/users/"+created.Sub+"/required_actions", csrf, cookie,
		map[string]any{"action": "teleport"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status=%d body=%s", bad.Code, bad.Body.String())
	}

	cleared := adminJSONRequest(t, e, http.MethodDelete,
		"/api/admin/users/"+created.Sub+"/required_actions/update_password", csrf, cookie, nil)
	if cleared.Code != http.StatusOK {
		t.Fatalf("clear status=%d body=%s", cleared.Code, cleared.Body.String())
	}
	user, err = repo.FindBySub(context.Background(), created.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || len(user.Lifecycle.RequiredActions) != 0 {
		t.Fatalf("required actions not cleared: %+v", user)
	}
}

func TestAdminUserAPIDeletesUserWithCascade(t *testing.T) {
	e, repo := newAdminUserHandler(t)
	csrf, cookie := adminCSRF(t, e)

	create := adminJSONRequest(t, e, http.MethodPost, "/api/admin/users", csrf, cookie, map[string]any{
		"preferred_username": "alice",
		"password":           "initial-password-9182",
		"email":              "alice@example.com",
		"roles":              []string{"support"},
	})
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	del := adminJSONRequest(t, e, http.MethodDelete, "/api/admin/users/"+created.Sub, csrf, cookie,
		map[string]any{"reason": "leaving company"})
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}

	// Tombstone is invisible to FindBySub.
	if user, _ := repo.FindBySub(context.Background(), created.Sub); user != nil {
		t.Fatalf("FindBySub returned deleted user: %+v", user)
	}
	tombstone, err := repo.FindBySubIncludingDeleted(context.Background(), created.Sub)
	if err != nil {
		t.Fatal(err)
	}
	if tombstone == nil || !tombstone.IsDeleted() {
		t.Fatalf("tombstone not persisted: %+v", tombstone)
	}
	if tombstone.PreferredUsername != "deleted:"+created.Sub || tombstone.Email != nil {
		t.Fatalf("PII not anonymized: %+v", tombstone)
	}

	// Listing no longer shows the user.
	list := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	list.Header.Set("X-Demo-Sub", "admin")
	listResp := httptest.NewRecorder()
	e.ServeHTTP(listResp, list)
	if strings.Contains(listResp.Body.String(), created.Sub) {
		t.Fatalf("deleted user still listed: %s", listResp.Body.String())
	}

	// Idempotent.
	again := adminJSONRequest(t, e, http.MethodDelete, "/api/admin/users/"+created.Sub, csrf, cookie, nil)
	if again.Code != http.StatusNoContent {
		t.Fatalf("idempotent delete status=%d body=%s", again.Code, again.Body.String())
	}
}

func TestAdminUserAPIRejectsSelfDelete(t *testing.T) {
	e, _ := newAdminUserHandler(t)
	csrf, cookie := adminCSRF(t, e)
	resp := adminJSONRequest(t, e, http.MethodDelete, "/api/admin/users/admin", csrf, cookie, nil)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "self_delete_forbidden") {
		t.Fatalf("unexpected body=%s", resp.Body.String())
	}
}

func newAdminUserHandler(
	t *testing.T,
) (*echo.Echo, *memory.UserRepository) {
	t.Helper()
	repo := memory.NewUserRepository()
	history := memory.NewPasswordHistoryRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	now := time.Now().UTC()
	for _, user := range []*spec.User{
		{
			Sub: "admin", PreferredUsername: "admin", PasswordHash: "unused",
			Roles: []string{"admin"}, CreatedAt: now, UpdatedAt: now,
		},
		{
			Sub: "regular", PreferredUsername: "regular", PasswordHash: "unused",
			CreatedAt: now, UpdatedAt: now,
		},
	} {
		repo.Seed(user)
	}
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer: "http://idp.test", UserRepo: repo, PasswordHasher: hasher,
		PasswordHistoryRepo: history, AuthnResolver: authusecases.DemoHeaderResolver{},
	})
	return e, repo
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
