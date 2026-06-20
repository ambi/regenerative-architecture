package http

// SCL scenario "テナント内 admin は所属テナントの custom 属性定義を読み・更新できる"
// を /api/admin/tenant/user_attribute_schema 経由で検証する (ADR-040 / wi-19)。

import (
	"bytes"
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

func newUserAttributeSchemaServer(
	t *testing.T, actor *spec.User, tenants ...*spec.Tenant,
) (*echo.Echo, *memory.TenantUserAttributeSchemaRepository, *[]spec.DomainEvent) {
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
	schemaRepo := memory.NewTenantUserAttributeSchemaRepository()
	resolver := &fakeAuthnResolver{}
	if actor != nil {
		resolver.ctx = &authdomain.AuthenticationContext{
			Sub: actor.Sub, AuthTime: time.Now().Unix(), AMR: []string{"pwd"},
		}
	}
	events := make([]spec.DomainEvent, 0)
	emit := func(e spec.DomainEvent) { events = append(events, e) }
	e := echo.New()
	Register(e, Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(), UserRepo: userRepo,
		TenantRepo: tenantRepo, AttrSchemaRepo: schemaRepo,
		AuthnResolver: resolver, Emit: emit,
	})
	return e, schemaRepo, &events
}

func putUserAttributeSchema(t *testing.T, e *echo.Echo, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	tenant := tenantPrefix(path)
	csrf, cookie := passwordResetContextCSRF(t, e, tenant+"/api/auth/password_reset_context")
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://idp.test")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestUserAttributeSchemaGetReturnsBuiltinForUndefinedTenant(t *testing.T) {
	e, _, _ := newUserAttributeSchemaServer(t, settingsActor("admin", "acme", []string{"admin"}), activeTenant("acme", "Acme"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/acme/api/admin/tenant/user_attribute_schema", http.NoBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body userAttributeSchemaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TenantID != "acme" || len(body.Attributes) != 0 {
		t.Fatalf("expected empty custom attributes, got %+v", body)
	}
	if len(body.Builtin) == 0 {
		t.Fatalf("expected builtin catalog to be returned")
	}
}

func TestUserAttributeSchemaGetRejectsNonAdmin(t *testing.T) {
	e, _, _ := newUserAttributeSchemaServer(t, settingsActor("alice", "acme", nil), activeTenant("acme", "Acme"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/realms/acme/api/admin/tenant/user_attribute_schema", http.NoBody))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUserAttributeSchemaPutPersistsAndEmitsEvent(t *testing.T) {
	e, schemaRepo, events := newUserAttributeSchemaServer(
		t, settingsActor("admin", "acme", []string{"admin"}), activeTenant("acme", "Acme"),
	)
	rec := putUserAttributeSchema(t, e, "/realms/acme/api/admin/tenant/user_attribute_schema", map[string]any{
		"attributes": []map[string]any{
			{"key": "region", "type": "string", "visibility": "claim_exposed", "claim_name": "region"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	stored, err := schemaRepo.FindByTenant(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || len(stored.Attributes) != 1 || stored.Attributes[0].Key != "region" {
		t.Fatalf("schema not persisted: %#v", stored)
	}
	found := false
	for _, ev := range *events {
		if ev.EventType() == "TenantUserAttributeSchemaUpdated" {
			found = true
		}
	}
	if !found {
		t.Fatalf("TenantUserAttributeSchemaUpdated not emitted: %+v", *events)
	}
}

func TestUserAttributeSchemaPutRejectsBuiltinCollision(t *testing.T) {
	e, _, _ := newUserAttributeSchemaServer(
		t, settingsActor("admin", "acme", []string{"admin"}), activeTenant("acme", "Acme"),
	)
	rec := putUserAttributeSchema(t, e, "/realms/acme/api/admin/tenant/user_attribute_schema", map[string]any{
		"attributes": []map[string]any{
			{"key": "nickname", "type": "string", "visibility": "claim_exposed"},
		},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
