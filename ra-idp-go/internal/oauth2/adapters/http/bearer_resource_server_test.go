package http_test

// ADR-061 / wi-66: OIDC RP 化した portal が提示する Bearer access token を
// resource server として受理する経路を /api/admin/policy/roles 経由で検証する。
// セッション resolver を配線せず、Bearer のみで admin 認可が成立すること、
// 無効トークン・非 admin・トークン無しの fail-closed を確認する。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	httpadapter "ra-idp-go/internal/infrastructure/http"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// stubIntrospector は token 文字列から固定の IntrospectionResult を返す。
type stubIntrospector struct {
	byToken map[string]*oauthports.IntrospectionResult
}

func (s stubIntrospector) IntrospectAccessToken(
	_ context.Context, token string,
) (*oauthports.IntrospectionResult, error) {
	if res, ok := s.byToken[token]; ok {
		return res, nil
	}
	return &oauthports.IntrospectionResult{Active: false}, nil
}

func newBearerAdminServer(t *testing.T, actor *spec.User, introspector oauthports.TokenIntrospector) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	if actor != nil {
		userRepo.Seed(actor)
	}
	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer: "http://idp.test", SCL: spec.MustLoadSCL(), UserRepo: userRepo,
		TokenIntrospector: introspector, TenantRepo: newSingleTenantRepo(),
		Emit: func(spec.DomainEvent) {},
	})
	return e
}

// adminRolePoliciesPath は RequireAdmin/ResolveAdminActor を通る代表的な admin GET。
const adminRolePoliciesPath = "/realms/acme/api/admin/policy/roles"

func getWithBearer(e *echo.Echo, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, adminRolePoliciesPath, http.NoBody)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func activeToken(sub string) *oauthports.IntrospectionResult {
	return tokenWithScope(sub, "openid ra.admin")
}

func tokenWithScope(sub, scope string) *oauthports.IntrospectionResult {
	return &oauthports.IntrospectionResult{Active: true, Sub: sub, Iat: time.Now().Unix(), Scope: scope}
}

func TestBearerAccessTokenAuthorizesAdmin(t *testing.T) {
	admin := keyAdminUser("user_alice", "acme", []string{"admin"})
	e := newBearerAdminServer(t, admin, stubIntrospector{byToken: map[string]*oauthports.IntrospectionResult{
		"good": activeToken("user_alice"),
	}})
	rec := getWithBearer(e, "good")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin Bearer should be authorized: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerMissingTokenIsUnauthorized(t *testing.T) {
	admin := keyAdminUser("user_alice", "acme", []string{"admin"})
	e := newBearerAdminServer(t, admin, stubIntrospector{})
	rec := getWithBearer(e, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credential must be 401: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerInactiveTokenIsUnauthorized(t *testing.T) {
	admin := keyAdminUser("user_alice", "acme", []string{"admin"})
	e := newBearerAdminServer(t, admin, stubIntrospector{}) // 未知 token は Active:false
	rec := getWithBearer(e, "revoked")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("inactive token must be 401: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerWithoutPortalScopeIsUnauthorized(t *testing.T) {
	admin := keyAdminUser("user_alice", "acme", []string{"admin"})
	// admin ロールはあるが token に ra.admin scope が無い → fail-closed (ADR-061)。
	e := newBearerAdminServer(t, admin, stubIntrospector{byToken: map[string]*oauthports.IntrospectionResult{
		"good": tokenWithScope("user_alice", "openid profile"),
	}})
	rec := getWithBearer(e, "good")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin API without ra.admin scope must be 401: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerAccountScopeRejectedOnAdminAPI(t *testing.T) {
	admin := keyAdminUser("user_alice", "acme", []string{"admin"})
	// account portal の token (ra.account) で admin API を叩く cross-portal 利用を拒否。
	e := newBearerAdminServer(t, admin, stubIntrospector{byToken: map[string]*oauthports.IntrospectionResult{
		"good": tokenWithScope("user_alice", "openid profile ra.account"),
	}})
	rec := getWithBearer(e, "good")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("account-scoped token must not authorize admin API: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerNonAdminIsForbidden(t *testing.T) {
	plain := keyAdminUser("user_bob", "acme", nil)
	e := newBearerAdminServer(t, plain, stubIntrospector{byToken: map[string]*oauthports.IntrospectionResult{
		"good": activeToken("user_bob"),
	}})
	rec := getWithBearer(e, "good")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin Bearer must be 403: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
