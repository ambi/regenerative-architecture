package http_test

// wi-43 / ADR-043: 高 sensitivity な self-service 操作は step-up 再認証を要求する。
// (1) 対象表 (パスワード変更 / MFA 解除 / email 変更 / 全セッション失効) の全ハンドラが、
//     recency 窓を外れたセッションに対し 403 step_up_required を返すことを表で照合する。
// (2) step_up/complete で再認証すると、同一セッションで gate を通過できる (flip) ことを
//     end-to-end で確認する。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authhttp "ra-idp-go/internal/authentication/adapters/http"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/crypto"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

const stepUpTestPassword = "demo-password-1234"

func activeTenant(id, displayName string) *spec.Tenant {
	return &spec.Tenant{
		ID: id, DisplayName: displayName, Status: spec.TenantStatusActive,
		CreatedAt: time.Now().UTC(),
	}
}

func newStepUpServer(t *testing.T) (*echo.Echo, *memory.SessionStore, *[]spec.DomainEvent) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()

	userRepo := memory.NewUserRepository()
	hasher := crypto.NewArgon2idPasswordHasher()
	hash, err := hasher.Hash(stepUpTestPassword)
	if err != nil {
		t.Fatal(err)
	}
	userRepo.Seed(&spec.User{
		Sub: "user-1", PreferredUsername: "alice", TenantID: spec.DefaultTenantID,
		PasswordHash: hash, MfaEnrolled: true,
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusActive},
		CreatedAt: now, UpdatedAt: now,
	})

	secret, err := authusecases.GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	mfaRepo := memory.NewMfaFactorRepository()
	if err := mfaRepo.Save(ctx, &spec.MfaFactor{
		Sub: "user-1", Type: spec.MfaFactorTOTP, Secret: &secret, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	tenantRepo := memory.NewTenantRepository()
	if err := tenantRepo.Save(ctx, activeTenant(spec.DefaultTenantID, "Default")); err != nil {
		t.Fatal(err)
	}

	store := memory.NewSessionStore()
	sm := authusecases.NewSessionManager(store)
	var events []spec.DomainEvent

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(),
		UserRepo: userRepo, TenantRepo: tenantRepo,
		AttrSchemaRepo:        memory.NewTenantUserAttributeSchemaRepository(),
		MfaFactorRepo:         mfaRepo,
		PasswordHasher:        hasher,
		PasswordHistoryRepo:   memory.NewPasswordHistoryRepository(),
		EmailChangeTokenStore: memory.NewEmailChangeTokenStore(),
		SessionManager:        sm, AuthnResolver: sm,
		Emit: func(ev spec.DomainEvent) { events = append(events, ev) },
	})
	return e, store, &events
}

// seedSession は指定した auth_time を持つ有効なセッション (step_up 未実施) を直接書き込み、
// その cookie 値 (session id) を返す。
func seedSession(t *testing.T, store *memory.SessionStore, id string, authTime time.Time) string {
	t.Helper()
	sess := &spec.LoginSession{
		ID: id, TenantID: spec.DefaultTenantID, Sub: "user-1",
		AuthTime: authTime.Unix(), AMR: []string{"pwd"},
		ACR:       authusecases.DeriveACR([]string{"pwd"}),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := store.Save(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	return id
}

func postAccount(e *echo.Echo, path, sessionID string, body any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://idp.test")
	req.Header.Set("X-CSRF-Token", "csrf-token-value")
	req.AddCookie(&http.Cookie{Name: core.CSRFCookie, Value: "csrf-token-value"})
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: authusecases.SessionCookie, Value: sessionID})
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		return ""
	}
	code, _ := body["error"].(string)
	return code
}

// 対象表: step-up が必要な sensitive 操作の全エンドポイント。
var stepUpGatedEndpoints = []struct {
	name string
	path string
}{
	{"change_password", "/realms/default/api/auth/change_password"},
	{"totp_remove", "/realms/default/api/account/mfa/totp/remove"},
	{"email_change", "/realms/default/api/account/email/change_request"},
	{"revoke_others", "/realms/default/api/account/sessions/revoke_others"},
}

// TestStepUpAnnotatedInterfacesMatchGatedHandlers は ADR-043 の対象表を機械照合する:
// SCL で step_up: required と注記した interface の http path 集合が、実装でゲートを
// 掛けたエンドポイント集合と完全に一致することを確認する。どちらかにズレが出れば失敗する。
func TestStepUpAnnotatedInterfacesMatchGatedHandlers(t *testing.T) {
	scl := spec.MustLoadSCL()
	sclPaths := map[string]bool{}
	for _, iface := range scl.Interfaces {
		if v, _ := iface.Annotations["step_up"].(string); v != "required" {
			continue
		}
		for _, b := range iface.Bindings {
			if kind, _ := b["kind"].(string); kind != "http" {
				continue
			}
			if p, _ := b["path"].(string); p != "" {
				sclPaths[p] = true
			}
		}
	}
	implPaths := map[string]bool{}
	for _, ep := range stepUpGatedEndpoints {
		implPaths[strings.TrimPrefix(ep.path, "/realms/default")] = true
	}
	if len(sclPaths) != len(implPaths) {
		t.Fatalf("step_up path count mismatch: scl=%v impl=%v", sclPaths, implPaths)
	}
	for p := range implPaths {
		if !sclPaths[p] {
			t.Fatalf("impl gates %q but SCL has no step_up annotation for it", p)
		}
	}
	for p := range sclPaths {
		if !implPaths[p] {
			t.Fatalf("SCL annotates %q step_up: required but no handler enforces it", p)
		}
	}
}

func TestStepUpGateBlocksStaleSessionOnAllSensitiveEndpoints(t *testing.T) {
	e, store, _ := newStepUpServer(t)
	stale := seedSession(t, store, "sess-stale", time.Now().Add(-10*time.Minute))
	for _, ep := range stepUpGatedEndpoints {
		rec := postAccount(e, ep.path, stale, map[string]any{})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: status=%d body=%s, want 403", ep.name, rec.Code, rec.Body.String())
		}
		if code := errorCode(t, rec); code != "step_up_required" {
			t.Fatalf("%s: error=%q, want step_up_required", ep.name, code)
		}
	}
}

func TestStepUpGateAllowsFreshSession(t *testing.T) {
	e, store, _ := newStepUpServer(t)
	fresh := seedSession(t, store, "sess-fresh", time.Now())
	for _, ep := range stepUpGatedEndpoints {
		rec := postAccount(e, ep.path, fresh, map[string]any{})
		// gate を通過するので step_up_required にはならない (以降の検証で別エラーや成功になる)。
		if code := errorCode(t, rec); code == "step_up_required" {
			t.Fatalf("%s: fresh session blocked by step-up (status=%d)", ep.name, rec.Code)
		}
	}
}

func TestStepUpStartReturnsAvailableMethods(t *testing.T) {
	e, store, _ := newStepUpServer(t)
	fresh := seedSession(t, store, "sess-fresh", time.Now())
	rec := postAccount(e, "/realms/default/api/account/step_up/start", fresh, map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body authhttp.StepUpStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Methods) != 2 || body.Methods[0] != "password" || body.Methods[1] != "totp" {
		t.Fatalf("methods=%v, want [password totp]", body.Methods)
	}
}

func TestStepUpCompleteFlipsGateForStaleSession(t *testing.T) {
	e, store, events := newStepUpServer(t)
	stale := seedSession(t, store, "sess-stale", time.Now().Add(-10*time.Minute))

	// 1. stale なので gate に弾かれる。
	rec := postAccount(e, "/realms/default/api/account/mfa/totp/remove", stale, map[string]any{"code": "000000"})
	if code := errorCode(t, rec); code != "step_up_required" {
		t.Fatalf("precondition: error=%q, want step_up_required", code)
	}

	// 2. パスワードで step-up を成立させる。
	rec = postAccount(e, "/realms/default/api/account/step_up/complete", stale,
		map[string]any{"method": "password", "password": stepUpTestPassword})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("complete: status=%d body=%s", rec.Code, rec.Body.String())
	}

	// 3. 同一セッションで gate を通過し、step-up 由来ではないエラー (不正コード) に進む。
	rec = postAccount(e, "/realms/default/api/account/mfa/totp/remove", stale, map[string]any{"code": "000000"})
	if code := errorCode(t, rec); code == "step_up_required" {
		t.Fatalf("gate did not flip after step-up: status=%d", rec.Code)
	}

	// StepUpCompleted が記録されている。
	found := false
	for _, ev := range *events {
		if c, ok := ev.(*spec.StepUpCompleted); ok && c.Method == "password" {
			found = true
		}
	}
	if !found {
		t.Fatal("StepUpCompleted not emitted")
	}
}

func TestStepUpCompleteWrongPasswordFails(t *testing.T) {
	e, store, _ := newStepUpServer(t)
	stale := seedSession(t, store, "sess-stale", time.Now().Add(-10*time.Minute))
	rec := postAccount(e, "/realms/default/api/account/step_up/complete", stale,
		map[string]any{"method": "password", "password": "wrong"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want 403", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "step_up_failed" {
		t.Fatalf("error=%q, want step_up_failed", code)
	}
}
