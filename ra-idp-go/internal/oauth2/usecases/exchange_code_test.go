package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

type fakeTokenIssuer struct {
	idTokenCalls int
}

func (f *fakeTokenIssuer) SignAccessToken(context.Context, ports.AccessTokenInput) (string, string, error) {
	return "access-token", "jti-1", nil
}

func (f *fakeTokenIssuer) SignIDToken(context.Context, ports.IDTokenInput) (string, error) {
	f.idTokenCalls++
	return "id-token", nil
}

func (f *fakeTokenIssuer) AccessTokenTTLSeconds() int { return 600 }
func (f *fakeTokenIssuer) IDTokenTTLSeconds() int     { return 3600 }

type exchangeFixture struct {
	deps         ExchangeCodeDeps
	codeStore    *memory.AuthorizationCodeStore
	refreshStore *memory.RefreshTokenStore
	code         *spec.AuthorizationCodeRecord
	issuer       *fakeTokenIssuer
}

func newExchangeFixture(t *testing.T, scopes []string) exchangeFixture {
	t.Helper()
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	codeStore := memory.NewAuthorizationCodeStore()
	refreshStore := memory.NewRefreshTokenStore()
	issuer := &fakeTokenIssuer{}

	now := time.Now().UTC()
	clientRepo.Seed(&spec.Client{
		ClientID: "client", ClientType: spec.ClientConfidential,
		RedirectURIs: []string{"https://client.example/cb"},
		GrantTypes:   []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
		ResponseTypes: []spec.ResponseType{
			spec.ResponseTypeCode,
		},
		TokenEndpointAuthMethod:  spec.AuthMethodClientSecretBasic,
		Scope:                    "openid profile offline_access",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user", PreferredUsername: "alice", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})

	verifier := "verifier-of-sufficient-length-ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum := sha256.Sum256([]byte(verifier))
	code := &spec.AuthorizationCodeRecord{
		Code:                   "authorization-code",
		AuthorizationRequestID: "00000000-0000-4000-8000-000000000001",
		ClientID:               "client",
		Sub:                    "user",
		Scopes:                 scopes,
		RedirectURI:            "https://client.example/cb",
		CodeChallenge:          base64.RawURLEncoding.EncodeToString(sum[:]),
		CodeChallengeMethod:    spec.CodeChallengeMethodS256,
		AuthTime:               now.Unix(),
		State:                  spec.AuthCodeRecordIssued,
		IssuedAt:               now,
		ExpiresAt:              now.Add(time.Minute),
	}
	if err := codeStore.Save(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	return exchangeFixture{
		deps: ExchangeCodeDeps{
			ClientRepo: clientRepo, UserRepo: userRepo, CodeStore: codeStore,
			RefreshStore: refreshStore, TokenIssuer: issuer,
		},
		codeStore: codeStore, refreshStore: refreshStore, code: code, issuer: issuer,
	}
}

func exchangeInput(verifier string) ExchangeCodeInput {
	return ExchangeCodeInput{
		ClientID: "client", Code: "authorization-code",
		CodeVerifier: verifier, RedirectURI: "https://client.example/cb",
	}
}

func TestExchangeCodePKCEFailureDoesNotConsumeCode(t *testing.T) {
	f := newExchangeFixture(t, []string{"openid"})
	if _, err := ExchangeCodeForToken(context.Background(), f.deps, exchangeInput("wrong-verifier")); err == nil {
		t.Fatal("expected PKCE failure")
	}

	out, err := ExchangeCodeForToken(
		context.Background(),
		f.deps,
		exchangeInput("verifier-of-sufficient-length-ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
	)
	if err != nil {
		t.Fatalf("valid retry failed: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatal("access token missing")
	}
}

func TestExchangeCodeReplayRevokesRefreshFamily(t *testing.T) {
	f := newExchangeFixture(t, []string{"openid", "offline_access"})
	out, err := ExchangeCodeForToken(
		context.Background(),
		f.deps,
		exchangeInput("verifier-of-sufficient-length-ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.RefreshToken == "" {
		t.Fatal("refresh token missing")
	}
	if _, err := ExchangeCodeForToken(
		context.Background(),
		f.deps,
		exchangeInput("verifier-of-sufficient-length-ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
	); err == nil {
		t.Fatal("expected replay rejection")
	}
	rec, err := f.refreshStore.FindByHash(context.Background(), domain.HashRefreshToken(out.RefreshToken))
	if err != nil {
		t.Fatal(err)
	}
	if rec == nil || !rec.Revoked {
		t.Fatal("refresh family was not revoked")
	}
}

func TestExchangeCodeIssuesTokensByScope(t *testing.T) {
	f := newExchangeFixture(t, []string{"profile"})
	out, err := ExchangeCodeForToken(
		context.Background(),
		f.deps,
		exchangeInput("verifier-of-sufficient-length-ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.IDToken != "" || f.issuer.idTokenCalls != 0 {
		t.Fatal("id_token must require openid scope")
	}
	if out.RefreshToken != "" {
		t.Fatal("refresh_token must require offline_access scope")
	}
}
