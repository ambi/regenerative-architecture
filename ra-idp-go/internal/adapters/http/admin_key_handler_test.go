package http

// SCL scenario "管理者は署名鍵を参照し、system_admin だけが default tenant 経路で
// ローテートできる" を /api/admin/keys 経由で検証する。
// - AdminKeysRead: admin / system_admin どちらでも List/Get 可能
// - AdminKeysRotate: system_admin + default tenant の二段絞り

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	"ra-idp-go/internal/adapters/persistence/memory"
	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func newKeyAdminServer(t *testing.T, actor *spec.User) (*echo.Echo, *crypto.InMemoryKeyStore, *[]spec.DomainEvent) {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if actor != nil {
		userRepo.Seed(actor)
	}
	keyStore, err := crypto.NewInMemoryKeyStore()
	if err != nil {
		t.Fatal(err)
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
	Register(e, Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(), UserRepo: userRepo,
		KeyStore: keyStore, AuthnResolver: resolver,
		TenantRepo: newSingleTenantRepo(),
		Emit:       emit,
	})
	return e, keyStore, &events
}

func keyAdminUser(sub, tenantID string, roles []string) *spec.User {
	now := time.Now().UTC()
	return &spec.User{
		Sub: sub, PreferredUsername: sub, TenantID: tenantID, Roles: roles,
		CreatedAt: now, UpdatedAt: now,
	}
}

func getAdminKeys(e *echo.Echo, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func postRotate(t *testing.T, e *echo.Echo, path string) *httptest.ResponseRecorder {
	t.Helper()
	// CSRF token / cookie は password_reset_context 経由で発行する。
	csrf, cookie := passwordResetContextCSRF(t, e, "/realms/default/api/auth/password_reset_context")
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://idp.test")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func passwordResetContextCSRF(t *testing.T, e *echo.Echo, path string) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("csrf bootstrap status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	result := rec.Result()
	defer result.Body.Close()
	cookies := result.Cookies()
	if len(cookies) == 0 || body.CSRFToken == "" {
		t.Fatalf("csrf=%q cookies=%v", body.CSRFToken, cookies)
	}
	return body.CSRFToken, cookies[0]
}

func TestAdminKeysListRequiresAdminRole(t *testing.T) {
	plain := keyAdminUser("user_alice", "acme", []string{})
	e, _, _ := newKeyAdminServer(t, plain)
	rec := getAdminKeys(e, "/realms/acme/api/admin/keys")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminKeysListReturnsAllKeys(t *testing.T) {
	user := keyAdminUser("user_admin", "acme", []string{"admin"})
	e, keyStore, _ := newKeyAdminServer(t, user)
	// 2 つ目の鍵を生成し JWKS 上に active+verifying を作る
	if _, err := keyStore.Rotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := getAdminKeys(e, "/realms/acme/api/admin/keys")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Keys []adminKeyResponse `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Keys) != 2 {
		t.Fatalf("keys=%d want 2", len(body.Keys))
	}
	active := 0
	for _, k := range body.Keys {
		if k.Active {
			active++
		}
		if _, ok := k.PublicJWK["n"]; !ok {
			t.Fatalf("public JWK must include RSA modulus n: %+v", k.PublicJWK)
		}
	}
	if active != 1 {
		t.Fatalf("exactly one active key expected, got %d", active)
	}
}

func TestAdminKeysGetUnknownKidReturns404(t *testing.T) {
	user := keyAdminUser("user_admin", "acme", []string{"admin"})
	e, _, _ := newKeyAdminServer(t, user)
	rec := getAdminKeys(e, "/realms/acme/api/admin/keys/unknown-kid")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminKeysRotateRejectsPlainAdmin(t *testing.T) {
	admin := keyAdminUser("user_admin", spec.DefaultTenantID, []string{"admin"})
	e, _, _ := newKeyAdminServer(t, admin)
	rec := postRotate(t, e, "/realms/default/api/admin/keys/rotate")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminKeysRotateRejectsSystemAdminOutsideDefaultPath(t *testing.T) {
	// system_admin (TenantID=default) が /realms/acme/.../rotate に到達した場合、
	// 二段の防御で reject される:
	//   1. resolveAuthentication が user.TenantID != requestTenantID(=acme) で
	//      セッションを未認証扱いし 401 を返す (defense-in-depth)
	//   2. もし 1 を抜けても requireKeyRotator が requestTenantID != default で 403
	// 期待される挙動は (1) が先に発火するため 401。
	sysAdmin := keyAdminUser("user_sys", spec.DefaultTenantID, []string{"system_admin"})
	e, _, _ := newKeyAdminServer(t, sysAdmin)
	rec := postRotate(t, e, "/realms/acme/api/admin/keys/rotate")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminKeysRotateSucceedsAndEmitsEvent(t *testing.T) {
	sysAdmin := keyAdminUser("user_sys", spec.DefaultTenantID, []string{"system_admin"})
	e, keyStore, events := newKeyAdminServer(t, sysAdmin)
	prevActive, err := keyStore.GetActiveKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rec := postRotate(t, e, "/realms/default/api/admin/keys/rotate")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body adminRotateKeyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Next.Active {
		t.Fatalf("next key must be active: %+v", body.Next)
	}
	if body.Previous == nil || body.Previous.Kid != prevActive.Kid {
		t.Fatalf("previous kid mismatch: prev=%+v want=%s", body.Previous, prevActive.Kid)
	}
	if body.Previous.Active {
		t.Fatalf("previous key must be non-active after rotation: %+v", body.Previous)
	}
	if len(*events) != 1 {
		t.Fatalf("expected 1 emitted event, got %d", len(*events))
	}
	if _, ok := (*events)[0].(*spec.SigningKeyRotated); !ok {
		t.Fatalf("event type=%T, want *spec.SigningKeyRotated", (*events)[0])
	}
}
