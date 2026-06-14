package usecases

// SCL シナリオ "absolute_expires_at を超えた refresh token はローテーション不可" と
// sender constraint 不一致 (DPoP / mTLS) で invalid_grant になることを担保する。

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

type refreshFixture struct {
	deps   RefreshDeps
	token  string
	record *spec.RefreshTokenRecord
}

func newRefreshFixture(t *testing.T, sc *spec.SenderConstraint, now time.Time, ttl time.Duration) refreshFixture {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	refreshStore := memory.NewRefreshTokenStore()
	issuer := &fakeTokenIssuer{}

	clientRepo.Seed(&spec.Client{
		ClientID: "client", ClientType: spec.ClientConfidential,
		RedirectURIs:             []string{"https://client.example/cb"},
		GrantTypes:               []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
		ResponseTypes:            []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		Scope:                    "openid offline_access",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user", PreferredUsername: "alice", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})

	gen, err := domain.GenerateInitialRefreshToken("client", "user", []string{"openid", "offline_access"}, sc, now)
	if err != nil {
		t.Fatal(err)
	}
	// 期限を上書きして AbsoluteExpiresAt の境界を意図した値に揃える。
	gen.Record.AbsoluteExpiresAt = now.Add(ttl)
	if err := refreshStore.Save(context.Background(), gen.Record); err != nil {
		t.Fatal(err)
	}

	return refreshFixture{
		deps: RefreshDeps{
			ClientRepo: clientRepo, UserRepo: userRepo,
			RefreshStore: refreshStore, TokenIssuer: issuer,
		},
		token:  gen.Token,
		record: gen.Record,
	}
}

func TestRefreshTokensRejectsAbsoluteTTLExpired(t *testing.T) {
	now := time.Now().UTC()
	// AbsoluteExpiresAt を過去にしてローテーション不可を観測する (ADR-004)。
	f := newRefreshFixture(t, nil, now, -time.Minute)
	_, err := RefreshTokens(context.Background(), f.deps, RefreshInput{
		ClientID: "client", RefreshToken: f.token,
	}, now)
	if err == nil {
		t.Fatal("expected absolute_expires_at rejection")
	}
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "invalid_grant" {
		t.Fatalf("expected invalid_grant, got %v", err)
	}
}

func TestRefreshTokensRejectsDPoPSenderConstraintMismatch(t *testing.T) {
	now := time.Now().UTC()
	sc := &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: "expected-jkt"}
	f := newRefreshFixture(t, sc, now, time.Hour)
	_, err := RefreshTokens(context.Background(), f.deps, RefreshInput{
		ClientID:     "client",
		RefreshToken: f.token,
		ProofJKT:     "different-jkt",
	}, now)
	if err == nil {
		t.Fatal("expected DPoP sender constraint rejection")
	}
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "invalid_grant" {
		t.Fatalf("expected invalid_grant, got %v", err)
	}
}

func TestRefreshTokensRejectsMTLSSenderConstraintMismatch(t *testing.T) {
	now := time.Now().UTC()
	sc := &spec.SenderConstraint{Type: spec.SenderConstraintMTLS, X5TS256: "expected-thumbprint"}
	f := newRefreshFixture(t, sc, now, time.Hour)
	_, err := RefreshTokens(context.Background(), f.deps, RefreshInput{
		ClientID:     "client",
		RefreshToken: f.token,
		ProofX5TS256: "attacker-thumbprint",
	}, now)
	if err == nil {
		t.Fatal("expected mTLS sender constraint rejection")
	}
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "invalid_grant" {
		t.Fatalf("expected invalid_grant, got %v", err)
	}
}

func TestRefreshTokensAcceptsMatchingDPoPProof(t *testing.T) {
	now := time.Now().UTC()
	sc := &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: "matching-jkt"}
	f := newRefreshFixture(t, sc, now, time.Hour)
	// tenant context が無いと FindByID は default を期待するが、Seed では明示せず
	// memory.ClientRepository が空 tenant_id でマッチするため通る。
	res, err := RefreshTokens(
		tenancy.WithTenant(context.Background(), &spec.Tenant{ID: f.record.TenantID, Status: spec.TenantStatusActive}, "", ""),
		f.deps,
		RefreshInput{ClientID: "client", RefreshToken: f.token, ProofJKT: "matching-jkt"},
		now,
	)
	if err != nil {
		t.Fatalf("matching proof rejected: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected rotated tokens, got %+v", res)
	}
}
