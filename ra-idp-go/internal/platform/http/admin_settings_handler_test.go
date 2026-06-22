package http

// SCL scenario "テナント内 admin は所属テナントの設定を読み・更新できる"
// を /api/admin/settings 経由で検証する。AdminSettingsRead は admin /
// system_admin の両方で許可、AdminSettingsUpdate は actor.tenant_id に
// 固定する。password_policy_override の弱化は use case 側で reject される。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"
	tenancyhttp "ra-idp-go/internal/tenancy/adapters/http"

	"github.com/labstack/echo/v5"
)

func newSettingsServer(t *testing.T, actor *spec.User, tenants ...*spec.Tenant) (*echo.Echo, *memory.TenantRepository, *[]spec.DomainEvent) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if actor != nil {
		userRepo.Seed(actor)
	}
	tenantRepo := memory.NewTenantRepository()
	for _, tenant := range tenants {
		if err := tenantRepo.Save(context.Background(), tenant); err != nil {
			t.Fatal(err)
		}
	}
	resolver := &fakeAuthnResolver{}
	if actor != nil {
		resolver.ctx = &authdomain.AuthenticationContext{
			Sub: actor.Sub, AuthTime: time.Now().Unix(), AMR: []string{"pwd"},
		}
	}
	events := make([]spec.DomainEvent, 0)
	emit := func(e spec.DomainEvent) { events = append(events, e) }
	e := echo.New()
	Register(e, core.Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(), UserRepo: userRepo,
		TenantRepo:    tenantRepo,
		AuthnResolver: resolver, Emit: emit,
	})
	return e, tenantRepo, &events
}

func settingsActor(sub, tenantID string, roles []string) *spec.User {
	now := time.Now().UTC()
	return &spec.User{
		Sub: sub, PreferredUsername: sub, TenantID: tenantID, Roles: roles,
		CreatedAt: now, UpdatedAt: now,
	}
}

func activeTenant(id, displayName string) *spec.Tenant {
	return &spec.Tenant{
		ID: id, DisplayName: displayName, Status: spec.TenantStatusActive,
		CreatedAt: time.Now().UTC(),
	}
}

func TestAdminSettingsGetRejectsNonAdmin(t *testing.T) {
	e, _, _ := newSettingsServer(t, settingsActor("alice", "acme", nil), activeTenant("acme", "Acme"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/acme/api/admin/settings", http.NoBody))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminSettingsGetReturnsCurrentTenant(t *testing.T) {
	minLength := 16
	tenant := activeTenant("acme", "Acme")
	tenant.PasswordPolicyOverride = &spec.PasswordPolicyOverride{MinLength: &minLength}
	e, _, _ := newSettingsServer(t, settingsActor("admin", "acme", []string{"admin"}), tenant)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/acme/api/admin/settings", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body tenancyhttp.AdminSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TenantID != "acme" || body.DisplayName != "Acme" {
		t.Fatalf("body=%+v", body)
	}
	if body.PasswordPolicyOverride == nil || body.PasswordPolicyOverride.MinLength == nil ||
		*body.PasswordPolicyOverride.MinLength != minLength {
		t.Fatalf("override=%+v", body.PasswordPolicyOverride)
	}
	if body.PasswordPolicyDefaults.MinLength <= 0 ||
		body.PasswordPolicyDefaults.MaxLength <= 0 ||
		body.PasswordPolicyDefaults.HistoryDepth <= 0 {
		t.Fatalf("defaults must be populated: %+v", body.PasswordPolicyDefaults)
	}
}

func TestAdminSettingsGetAllowsSystemAdmin(t *testing.T) {
	e, _, _ := newSettingsServer(
		t,
		settingsActor("ops", spec.DefaultTenantID, []string{"system_admin"}),
		activeTenant(spec.DefaultTenantID, "Default"),
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/default/api/admin/settings", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminSettingsPatchUpdatesAndEmitsEvent(t *testing.T) {
	e, repo, events := newSettingsServer(
		t,
		settingsActor("admin", "acme", []string{"admin"}),
		activeTenant("acme", "Acme"),
	)
	resp := patchSettings(t, e, "/realms/acme/api/admin/settings", map[string]any{
		"display_name": "Acme Inc.",
		"password_policy_override": map[string]int{
			"min_length": 16, "history_depth": 10,
		},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	tenant, err := repo.FindByID(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if tenant.DisplayName != "Acme Inc." {
		t.Fatalf("display_name=%q", tenant.DisplayName)
	}
	if tenant.PasswordPolicyOverride == nil ||
		tenant.PasswordPolicyOverride.MinLength == nil ||
		*tenant.PasswordPolicyOverride.MinLength != 16 {
		t.Fatalf("override=%+v", tenant.PasswordPolicyOverride)
	}
	if len(*events) != 1 {
		t.Fatalf("events=%d want 1", len(*events))
	}
	updated, ok := (*events)[0].(*spec.TenantUpdated)
	if !ok {
		t.Fatalf("event type=%T", (*events)[0])
	}
	if updated.TenantID != "acme" {
		t.Fatalf("event tenant=%q", updated.TenantID)
	}
}

func TestAdminSettingsPatchRejectsWeakerPolicy(t *testing.T) {
	e, _, _ := newSettingsServer(
		t,
		settingsActor("admin", "acme", []string{"admin"}),
		activeTenant("acme", "Acme"),
	)
	resp := patchSettings(t, e, "/realms/acme/api/admin/settings", map[string]any{
		"password_policy_override": map[string]int{"min_length": 4},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("policy_override_weaker")) {
		t.Fatalf("unexpected body=%s", resp.Body.String())
	}
}

// admin が /realms/{自テナント}/api/admin/settings から触れるのは自テナントのみで、
// 別テナントを書き換える経路は存在しない。
func TestAdminSettingsPatchStaysWithinActorTenant(t *testing.T) {
	e, repo, _ := newSettingsServer(
		t,
		settingsActor("admin", "acme", []string{"admin"}),
		activeTenant("acme", "Acme"),
		activeTenant("other", "Other"),
	)
	resp := patchSettings(t, e, "/realms/acme/api/admin/settings", map[string]any{
		"display_name": "Modified",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	other, err := repo.FindByID(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	if other.DisplayName != "Other" {
		t.Fatalf("other tenant was modified: %q", other.DisplayName)
	}
}

func TestAdminSettingsPatchRequiresCSRF(t *testing.T) {
	e, _, _ := newSettingsServer(
		t,
		settingsActor("admin", "acme", []string{"admin"}),
		activeTenant("acme", "Acme"),
	)
	body, _ := json.Marshal(map[string]string{"display_name": "X"})
	req := httptest.NewRequest(http.MethodPatch, "/realms/acme/api/admin/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://idp.test")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("status=%d (CSRF should reject)", rec.Code)
	}
}

func patchSettings(t *testing.T, e *echo.Echo, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	// CSRF token / cookie を tenant local の password_reset_context 経由で発行する。
	tenant := tenantPrefix(path)
	csrf, cookie := passwordResetContextCSRF(t, e, tenant+"/api/auth/password_reset_context")
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://idp.test")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func tenantPrefix(path string) string {
	// "/realms/acme/api/admin/settings" -> "/realms/acme"
	const prefix = "/realms/"
	if len(path) < len(prefix) || path[:len(prefix)] != prefix {
		return ""
	}
	rest := path[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			return prefix + rest[:i]
		}
	}
	return prefix + rest
}
