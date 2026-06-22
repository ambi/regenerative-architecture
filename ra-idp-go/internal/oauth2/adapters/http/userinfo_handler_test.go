package http_test

// /userinfo + DPoP PoP 検証 (RFC 9449 §7) のテスト。SenderConstraintDPoP の AT は
// 同じ鍵で署名された DPoP proof と htm / htu / iat / jti を伴わない限り受理しない。

import (
	"bytes"
	"context"
	cryptostd "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	httpadapter "ra-idp-go/internal/platform/http"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type fakeIntrospector struct {
	result *oauthports.IntrospectionResult
}

func (f *fakeIntrospector) IntrospectAccessToken(_ context.Context, _ string) (*oauthports.IntrospectionResult, error) {
	return f.result, nil
}

func rsaJWK(pub *rsa.PublicKey) map[string]any {
	return map[string]any{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(pub.E)).Bytes()),
	}
}

func rsaJWKThumbprint(jwk map[string]any) string {
	// RFC 7638: required メンバーのみ、辞書順、空白なしの canonical JSON を SHA-256。
	canonical, _ := json.Marshal(map[string]any{"e": jwk["e"], "kty": jwk["kty"], "n": jwk["n"]})
	sum := sha256.Sum256(canonical)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func signDPoPProof(t *testing.T, key *rsa.PrivateKey, jwk map[string]any, htm, htu, jti string, now time.Time) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"typ": "dpop+jwt", "alg": "PS256", "jwk": jwk})
	payload, _ := json.Marshal(map[string]any{"htm": htm, "htu": htu, "jti": jti, "iat": now.Unix()})
	input := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPSS(
		rand.Reader, key, cryptostd.SHA256, digest[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestUserInfoDPoPBoundRequiresMatchingProof(t *testing.T) {
	now := time.Now().UTC()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := rsaJWK(&key.PublicKey)
	jkt := rsaJWKThumbprint(jwk)

	userRepo := memory.NewUserRepository()
	userRepo.Seed(&spec.User{
		Sub: "user_alice", PreferredUsername: "alice", TenantID: spec.DefaultTenantID,
		CreatedAt: now, UpdatedAt: now,
	})

	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, Sub: "user_alice", Scope: "openid profile",
		ClientID:         "demo-client",
		SenderConstraint: &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: jkt},
	}}

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:            "http://test",
		UserRepo:          userRepo,
		TokenIntrospector: intro,
		DpopReplayStore:   memory.NewDpopReplayStore(),
	})

	call := func(authHeader, proof string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/userinfo", http.NoBody)
		req.Header.Set("Authorization", authHeader)
		if proof != "" {
			req.Header.Set("DPoP", proof)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	mustReject := func(name string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code == http.StatusOK ||
			!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"invalid_token"`)) {
			t.Fatalf("%s: status=%d body=%s", name, rec.Code, rec.Body.String())
		}
	}

	// 有効プルーフ。htu は requestHTU と一致する形 (base + path)。
	validProof := signDPoPProof(t, key, jwk, "GET", "http://test/userinfo", "jti-valid", now)
	if rec := call("DPoP atoken", validProof); rec.Code != http.StatusOK {
		t.Fatalf("valid proof status=%d body=%s", rec.Code, rec.Body.String())
	}

	// DPoP ヘッダー欠落 → invalid_token。
	mustReject("missing DPoP proof", call("DPoP atoken", ""))

	// 別鍵で署名された proof → 署名検証は通っても jkt が一致せず invalid_token。
	attacker, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	attJWK := rsaJWK(&attacker.PublicKey)
	attProof := signDPoPProof(t, attacker, attJWK, "GET", "http://test/userinfo", "jti-attacker", now)
	mustReject("wrong jkt", call("DPoP atoken", attProof))
}

func TestUserInfoDPoPHTUUsesTenantPrefix(t *testing.T) {
	// /realms/{tenant}/userinfo にアクセスした場合、DPoP proof の htu に
	// テナント prefix が含まれていれば受理される (Phase 1 #3 の回帰防止)。
	now := time.Now().UTC()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := rsaJWK(&key.PublicKey)
	jkt := rsaJWKThumbprint(jwk)

	userRepo := memory.NewUserRepository()
	userRepo.Seed(&spec.User{
		Sub: "user_bob", PreferredUsername: "bob", TenantID: "acme",
		CreatedAt: now, UpdatedAt: now,
	})

	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, Sub: "user_bob", Scope: "openid", ClientID: "tenant-client",
		SenderConstraint: &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: jkt},
	}}

	// "acme" テナントを返す TenantRepository をその場で組む。
	tenantRepo := newSingleTenantRepo()

	e := echo.New()
	httpadapter.Register(e, core.Deps{
		Issuer:            "http://test",
		TenantRepo:        tenantRepo,
		UserRepo:          userRepo,
		TokenIntrospector: intro,
		DpopReplayStore:   memory.NewDpopReplayStore(),
	})

	htu := "http://test/realms/acme/userinfo"
	proof := signDPoPProof(t, key, jwk, "GET", htu, "jti-acme", now)

	req := httptest.NewRequest(http.MethodGet, "/realms/acme/userinfo", http.NoBody)
	req.Header.Set("Authorization", "DPoP atoken")
	req.Header.Set("DPoP", proof)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant-prefixed userinfo status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// newSingleTenantRepo は指定 ID の Active テナントだけを返す最小の TenantRepository。
type singleTenantRepo struct {
	tenant *spec.Tenant
}

func newSingleTenantRepo() *singleTenantRepo {
	now := time.Now().UTC()
	return &singleTenantRepo{tenant: &spec.Tenant{
		ID: "acme", Status: spec.TenantStatusActive, CreatedAt: now,
	}}
}

func (r *singleTenantRepo) FindByID(_ context.Context, id string) (*spec.Tenant, error) {
	if r.tenant.ID == id {
		return r.tenant, nil
	}
	if id == spec.DefaultTenantID {
		return &spec.Tenant{ID: spec.DefaultTenantID, Status: spec.TenantStatusActive}, nil
	}
	return nil, nil
}

func (r *singleTenantRepo) FindAll(_ context.Context) ([]*spec.Tenant, error) {
	return []*spec.Tenant{r.tenant}, nil
}

func (r *singleTenantRepo) Save(_ context.Context, _ *spec.Tenant) error { return nil }
func (r *singleTenantRepo) Delete(_ context.Context, _ string) error     { return nil }

// fakeDenylist は AccessToken denylist を任意の jti セットで再現する。
type fakeDenylist struct {
	revoked map[string]bool
}

func (f *fakeDenylist) Add(_ context.Context, jti string, _ time.Time) error {
	if f.revoked == nil {
		f.revoked = map[string]bool{}
	}
	f.revoked[jti] = true
	return nil
}

func (f *fakeDenylist) IsRevoked(_ context.Context, jti string) (bool, error) {
	return f.revoked[jti], nil
}

func newUserInfoServer(t *testing.T, intro *fakeIntrospector, denylist *fakeDenylist) *echo.Echo {
	t.Helper()
	userRepo := memory.NewUserRepository()
	now := time.Now().UTC()
	userRepo.Seed(&spec.User{
		Sub: "user_alice", PreferredUsername: "alice",
		TenantID: spec.DefaultTenantID, CreatedAt: now, UpdatedAt: now,
	})
	e := echo.New()
	deps := core.Deps{
		Issuer:            "http://test",
		UserRepo:          userRepo,
		TokenIntrospector: intro,
		DpopReplayStore:   memory.NewDpopReplayStore(),
	}
	if denylist != nil {
		deps.AccessTokenDenylist = denylist
	}
	httpadapter.Register(e, deps)
	return e
}

func TestUserInfoRejectsTokenWithoutOpenIDScope(t *testing.T) {
	// SCL シナリオ "openid スコープのないトークンのユーザー情報取得は拒否される"。
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, Sub: "user_alice", Scope: "profile", ClientID: "demo-client",
	}}
	e := newUserInfoServer(t, intro, nil)
	req := httptest.NewRequest(http.MethodGet, "/userinfo", http.NoBody)
	req.Header.Set("Authorization", "Bearer atoken")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"insufficient_scope"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUserInfoRejectsRevokedAccessToken(t *testing.T) {
	// SCL シナリオ "失効した access_token でユーザー情報取得は invalid_token で拒否される"。
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, Sub: "user_alice", Scope: "openid", ClientID: "demo-client",
		JTI: "revoked-jti",
	}}
	denylist := &fakeDenylist{revoked: map[string]bool{"revoked-jti": true}}
	e := newUserInfoServer(t, intro, denylist)
	req := httptest.NewRequest(http.MethodGet, "/userinfo", http.NoBody)
	req.Header.Set("Authorization", "Bearer atoken")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"invalid_token"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUserInfoMTLSBoundRequiresMatchingThumbprint(t *testing.T) {
	// SCL シナリオ "mTLS バインド AT は同じ証明書のリクエストでのみ受理される"。
	// 期待 thumbprint と異なる証明書を提示すると invalid_token。
	intro := &fakeIntrospector{result: &oauthports.IntrospectionResult{
		Active: true, Sub: "user_alice", Scope: "openid", ClientID: "demo-client",
		SenderConstraint: &spec.SenderConstraint{
			Type: spec.SenderConstraintMTLS, X5TS256: "expected-thumbprint-not-matching-any-real-cert",
		},
	}}
	e := newUserInfoServer(t, intro, nil)
	req := httptest.NewRequest(http.MethodGet, "/userinfo", http.NoBody)
	req.Header.Set("Authorization", "Bearer atoken")
	req.Header.Set("X-Client-Certificate", clientCertificateHeader(t, "client"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"error":"invalid_token"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// clientCertificateHeader はテスト用の自己署名証明書を PEM/URL エンコードして返す。
// router 経由の mTLS 検証テストで X-Client-Certificate ヘッダーに用いる。
func clientCertificateHeader(t *testing.T, commonName string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return url.QueryEscape(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
}
