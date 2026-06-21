package http

// wi-44 / ADR-045: /api/admin/authentication_events の検索を検証する。期間 (from/to) は
// 必須 (全期間スキャン禁止)、kind で success/fail/aggregated に絞れ、別テナントは漏れない。

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	authdomain "ra-idp-go/internal/authentication/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newAuthEventAdminServer(t *testing.T, actor *spec.User, events []*oauthports.AuditEventRecord) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if actor != nil {
		userRepo.Seed(actor)
	}
	store := memory.NewAuditEventStore(0)
	for _, ev := range events {
		_ = store.Append(context.Background(), ev)
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
		AuditEventRepo: store, AuthnResolver: resolver,
		TenantRepo: newSingleTenantRepo(),
	})
	return e
}

func authEventRecord(tenantID, typ, sub string, at time.Time, payload map[string]any) *oauthports.AuditEventRecord {
	full := map[string]any{"sub": sub, "tenantId": tenantID, "type": typ}
	maps.Copy(full, payload)
	return &oauthports.AuditEventRecord{
		ID:       tenantID + ":" + typ + ":" + sub + ":" + at.Format(time.RFC3339Nano),
		TenantID: tenantID, Type: typ, OccurredAt: at, Payload: full,
	}
}

func decodeAuthEvents(t *testing.T, rec *httptest.ResponseRecorder) []adminAuditEventResponse {
	t.Helper()
	var body struct {
		Events []adminAuditEventResponse `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return body.Events
}

func authEventWindow(now time.Time) string {
	from := url.QueryEscape(now.Add(-time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(now.Add(time.Hour).Format(time.RFC3339))
	return "from=" + from + "&to=" + to
}

func TestAuthenticationEventsRequireDateRange(t *testing.T) {
	admin := auditUser("user_admin", "acme", []string{"admin"})
	e := newAuthEventAdminServer(t, admin, nil)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing from/to must be 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticationEventsRequireReaderRole(t *testing.T) {
	user := auditUser("user_alice", "acme", []string{})
	now := time.Now().UTC()
	e := newAuthEventAdminServer(t, user, nil)
	rec := getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events?"+authEventWindow(now))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin must be 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticationEventsFilterByKindAndTenant(t *testing.T) {
	admin := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		authEventRecord("acme", "UserAuthenticated", "alice", now.Add(-time.Minute), nil),
		authEventRecord("acme", "AuthenticationFailed", "", now.Add(-2*time.Minute), map[string]any{"usernameHash": "h1"}),
		authEventRecord("acme", "PasswordChanged", "alice", now.Add(-3*time.Minute), nil), // 認証イベント外
		authEventRecord("default", "UserAuthenticated", "ops", now.Add(-time.Minute), nil),
	}
	e := newAuthEventAdminServer(t, admin, events)

	// kind 未指定: 認証イベントのみ・自テナントのみ (UserAuthenticated + AuthenticationFailed = 2)。
	all := decodeAuthEvents(t, getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events?"+authEventWindow(now)))
	if len(all) != 2 {
		t.Fatalf("kind unset must return 2 auth events in tenant, got %d: %+v", len(all), all)
	}
	for _, ev := range all {
		if ev.TenantID != "acme" || ev.Type == "PasswordChanged" {
			t.Fatalf("unexpected event leaked: %+v", ev)
		}
	}

	// kind=fail: AuthenticationFailed のみ。
	fails := decodeAuthEvents(t, getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events?kind=fail&"+authEventWindow(now)))
	if len(fails) != 1 || fails[0].Type != "AuthenticationFailed" {
		t.Fatalf("kind=fail mismatch: %+v", fails)
	}
}

func TestAuthenticationEventsFilterByUsernameHash(t *testing.T) {
	admin := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		authEventRecord("acme", "AuthenticationFailed", "", now.Add(-time.Minute), map[string]any{"usernameHash": "wanted"}),
		authEventRecord("acme", "AuthenticationFailed", "", now.Add(-2*time.Minute), map[string]any{"usernameHash": "other"}),
	}
	e := newAuthEventAdminServer(t, admin, events)
	got := decodeAuthEvents(t, getAdminAuditEvents(e,
		"/realms/acme/api/admin/authentication_events?username_hash=wanted&"+authEventWindow(now)))
	if len(got) != 1 || got[0].Payload["usernameHash"] != "wanted" {
		t.Fatalf("username_hash filter mismatch: %+v", got)
	}
}

func TestAuthenticationEventGetHidesNonAuthType(t *testing.T) {
	admin := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	rec := authEventRecord("acme", "PasswordChanged", "alice", now, nil)
	e := newAuthEventAdminServer(t, admin, []*oauthports.AuditEventRecord{rec})
	resp := getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events/"+url.PathEscape(rec.ID))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("non-auth event must be 404, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAuthenticationEventsExportSetsAttachment(t *testing.T) {
	admin := auditUser("user_admin", "acme", []string{"admin"})
	now := time.Now().UTC()
	events := []*oauthports.AuditEventRecord{
		authEventRecord("acme", "UserAuthenticated", "alice", now.Add(-time.Minute), nil),
	}
	e := newAuthEventAdminServer(t, admin, events)
	resp := getAdminAuditEvents(e, "/realms/acme/api/admin/authentication_events/export?"+authEventWindow(now))
	if resp.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", resp.Code, resp.Body.String())
	}
	if cd := resp.Header().Get("Content-Disposition"); cd == "" {
		t.Fatal("export must set Content-Disposition")
	}
	if got := decodeAuthEvents(t, resp); len(got) != 1 {
		t.Fatalf("export must return 1 event, got %d", len(got))
	}
}
