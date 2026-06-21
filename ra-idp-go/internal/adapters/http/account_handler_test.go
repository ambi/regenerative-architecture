package http

// SCL scenario "認証済みユーザーは自身のプロフィールを読み・編集できる" を
// /api/account/profile 経由で検証する (ADR-040 / wi-19)。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newAccountServer(t *testing.T, user *spec.User) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if user != nil {
		userRepo.Seed(user)
	}
	tenantRepo := memory.NewTenantRepository()
	if err := tenantRepo.Save(context.Background(), activeTenant(spec.DefaultTenantID, "Default")); err != nil {
		t.Fatal(err)
	}
	resolver := &fakeAuthnResolver{}
	if user != nil {
		resolver.ctx = &authdomain.AuthenticationContext{
			Sub: user.Sub, AuthTime: time.Now().Unix(), AMR: []string{"pwd"},
		}
	}
	e := echo.New()
	Register(e, Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(), UserRepo: userRepo,
		TenantRepo: tenantRepo, AttrSchemaRepo: memory.NewTenantUserAttributeSchemaRepository(),
		AuthnResolver: resolver, Emit: func(spec.DomainEvent) {},
	})
	return e
}

func accountUser() *spec.User {
	now := time.Now().UTC()
	name := "Dave Q"
	return &spec.User{
		Sub: "user-1", PreferredUsername: "dave", TenantID: spec.DefaultTenantID, Name: &name,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2g",
		Lifecycle:    spec.UserLifecycle{Status: spec.UserStatusActive},
		Attributes: map[string]spec.AttributeValue{
			"nickname":   {Type: spec.AttributeTypeString, String: ptrString("davey")},    // claim_exposed
			"department": {Type: spec.AttributeTypeString, String: ptrString("Platform")}, // self_readable
		},
		CreatedAt: now, UpdatedAt: now,
	}
}

func ptrString(s string) *string { return &s }

func TestAccountProfileGetRequiresAuth(t *testing.T) {
	e := newAccountServer(t, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/default/api/account/profile", http.NoBody))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountProfileGetReturnsSelfView(t *testing.T) {
	e := newAccountServer(t, accountUser())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/default/api/account/profile", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body accountProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body.Attributes["nickname"]; !ok {
		t.Fatalf("claim_exposed nickname missing from self view: %+v", body.Attributes)
	}
	if v, ok := body.Attributes["department"]; !ok || v.String == nil || *v.String != "Platform" {
		t.Fatalf("self_readable department missing from self view: %+v", body.Attributes)
	}
	if len(body.ReadableAttributes) == 0 {
		t.Fatalf("readable_attributes should be populated")
	}
	if len(body.EditableAttributes) == 0 {
		t.Fatalf("editable_attributes should be populated")
	}
}

func TestAccountSummaryRequiresAuth(t *testing.T) {
	e := newAccountServer(t, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/default/api/account/summary", http.NoBody))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccountSummaryReturnsLifecycleAndOmitsRoles(t *testing.T) {
	user := accountUser()
	last := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	user.Lifecycle.LastLoginAt = &last
	user.Lifecycle.RequiredActions = []spec.RequiredAction{spec.RequiredActionUpdatePassword}
	user.Roles = []string{"admin"}
	e := newAccountServer(t, user)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/default/api/account/summary", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["roles"]; ok {
		t.Fatalf("summary must not expose roles: %+v", body)
	}
	if body["last_login_at"] == nil {
		t.Fatalf("last_login_at missing: %+v", body)
	}
	actions, ok := body["required_actions"].([]any)
	if !ok || len(actions) != 1 || actions[0] != string(spec.RequiredActionUpdatePassword) {
		t.Fatalf("required_actions not projected: %+v", body["required_actions"])
	}
}

func TestAccountProfilePatchUpdatesEditableAttribute(t *testing.T) {
	e := newAccountServer(t, accountUser())
	rec := patchSettings(t, e, "/realms/default/api/account/profile", map[string]any{
		"given_name": "Dave",
		"attributes": map[string]any{
			"nickname": map[string]any{"type": "string", "string": "newnick"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body accountProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.GivenName == nil || *body.GivenName != "Dave" {
		t.Fatalf("given_name not updated: %+v", body.GivenName)
	}
	if v := body.Attributes["nickname"]; v.String == nil || *v.String != "newnick" {
		t.Fatalf("nickname not updated: %+v", body.Attributes)
	}
}

func TestAccountProfilePatchRejectsAdminManagedAttribute(t *testing.T) {
	e := newAccountServer(t, accountUser())
	rec := patchSettings(t, e, "/realms/default/api/account/profile", map[string]any{
		"attributes": map[string]any{
			"department": map[string]any{"type": "string", "string": "Sales"},
		},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
