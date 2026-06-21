package http

// SCL scenario "管理者は所属テナントの監査イベントを参照できるが別テナントは公開しない" を
// /api/admin/audit_events 経由で検証する。requireAdmin と異なり requireAuditReader は
// admin / system_admin 両方を許可し、system_admin の default-tenant 経路では
// all_tenants=true で横断検索できる。

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
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newAuditAdminServer(t *testing.T, actor *spec.User, events []*oauthports.AuditEventRecord) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if actor != nil {
		userRepo.Seed(actor)
	}
	auditStore := memory.NewAuditEventStore(0)
	for _, ev := range events {
		_ = auditStore.Append(context.Background(), ev)
	}
	resolver := &fakeAuthnResolver{}
	if actor != nil {
		resolver.ctx = &authdomain.AuthenticationContext{
			Sub: actor.Sub, AuthTime: time.Now().Unix(), AMR: []string{"pwd"},
		}
	}
	e := echo.New()
	Register(e, Deps{
		Issuer: "http://test", UserRepo: userRepo,
		AuditEventRepo: auditStore, AuthnResolver: resolver,
		TenantRepo: newSingleTenantRepo(),
	})
	return e
}

func auditUser(sub, tenantID string, roles []string) *spec.User {
	now := time.Now().UTC()
	return &spec.User{
		Sub: sub, PreferredUsername: sub, TenantID: tenantID, Roles: roles,
		CreatedAt: now, UpdatedAt: now,
	}
}

func auditEvent(tenantID, typ, sub string, occurredAt time.Time) *oauthports.AuditEventRecord {
	return &oauthports.AuditEventRecord{
		ID:       tenantID + ":" + typ + ":" + sub + ":" + occurredAt.Format(time.RFC3339Nano),
		TenantID: tenantID, Type: typ, OccurredAt: occurredAt,
		Payload: map[string]any{"sub": sub, "tenantId": tenantID, "type": typ},
	}
}

func getAdminAuditEvents(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAdminAuditEventsRequiresAdminRole(t *testing.T) {
	// 認証はあるが admin/system_admin ロールが無い → 403。
	user := auditUser("user_alice", "acme", []string{})
	e := newAuditAdminServer(t, user, nil)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events")
	if rec.Code != http.StatusForbidden ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"access_denied"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuditEventsScopesToOwnTenant(t *testing.T) {
	user := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "UserAuthenticated", "alice", now.Add(-time.Minute)),
		auditEvent("default", "UserAuthenticated", "ops", now.Add(-30*time.Second)),
		auditEvent("acme", "AccessTokenIssued", "alice", now),
	}
	e := newAuditAdminServer(t, user, events)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Events) != 2 {
		t.Fatalf("acme admin must see 2 events, got %d", len(body.Events))
	}
	for _, ev := range body.Events {
		if ev.TenantID != "acme" {
			t.Fatalf("cross-tenant leak: %+v", ev)
		}
	}
}

func TestAdminAuditEventsAllTenantsRequiresSystemAdminOnDefaultTenant(t *testing.T) {
	// admin (acme) が all_tenants=true を渡しても自テナント限定で動く。
	admin := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "X", "a", now),
		auditEvent("default", "X", "b", now),
	}
	e := newAuditAdminServer(t, admin, events)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events?all_tenants=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 1 || body.Events[0].TenantID != "acme" {
		t.Fatalf("admin must not escape own tenant: %+v", body.Events)
	}
}

func TestAdminAuditEventsAllTenantsHonoredForSystemAdminAtDefault(t *testing.T) {
	sysAdmin := auditUser("user_system_admin", spec.DefaultTenantID, []string{"system_admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "X", "a", now),
		auditEvent("default", "X", "b", now),
	}
	e := newAuditAdminServer(t, sysAdmin, events)
	rec := getAdminAuditEvents(e, "/realms/default/api/admin/audit_events?all_tenants=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 2 {
		t.Fatalf("system_admin all_tenants=true must see 2 events, got %d", len(body.Events))
	}
}

func TestAdminAuditEventsGetReturns404ForCrossTenant(t *testing.T) {
	user := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	foreign := auditEvent("default", "X", "alice", now)
	e := newAuditAdminServer(t, user, []*oauthports.AuditEventRecord{foreign})
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events/"+foreign.ID)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant event, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminAuditEventsFilterByTypeAndSub(t *testing.T) {
	user := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "UserAuthenticated", "alice", now),
		auditEvent("acme", "UserAuthenticated", "bob", now.Add(-time.Second)),
		auditEvent("acme", "AccessTokenIssued", "alice", now.Add(-2*time.Second)),
	}
	e := newAuditAdminServer(t, user, events)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events?type=UserAuthenticated&sub=alice")
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 1 ||
		body.Events[0].Type != "UserAuthenticated" ||
		body.Events[0].Payload["sub"] != "alice" {
		t.Fatalf("filter mismatch: %+v", body.Events)
	}
}

// wi-44 統合: 監査ログ検索に認証イベントの kind 絞り込みが乗っていることを検証する。
func TestAdminAuditEventsFilterByAuthenticationKind(t *testing.T) {
	user := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "UserAuthenticated", "alice", now),
		auditEvent("acme", "AuthenticationFailed", "", now.Add(-time.Second)),
		auditEvent("acme", "PasswordChanged", "alice", now.Add(-2*time.Second)), // 認証イベント外
	}
	e := newAuditAdminServer(t, user, events)

	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events?kind=fail")
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 1 || body.Events[0].Type != "AuthenticationFailed" {
		t.Fatalf("kind=fail mismatch: %+v", body.Events)
	}

	rec = getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events?kind=authentication")
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 2 {
		t.Fatalf("kind=authentication must exclude PasswordChanged, got %d: %+v", len(body.Events), body.Events)
	}

	rec = getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events?kind=bogus")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind must be 400, got %d", rec.Code)
	}
}

func TestAdminAuditEventsExportSetsAttachment(t *testing.T) {
	user := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		auditEvent("acme", "UserAuthenticated", "alice", now),
	}
	e := newAuditAdminServer(t, user, events)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/audit_events/export?kind=authentication")
	if rec.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Fatal("export must set Content-Disposition")
	}
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if len(body.Events) != 1 {
		t.Fatalf("export must return 1 event, got %d", len(body.Events))
	}
}
