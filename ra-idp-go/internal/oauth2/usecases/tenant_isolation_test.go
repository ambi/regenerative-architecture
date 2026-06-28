package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/infrastructure/persistence/memory"
	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

func tenantContext(id string) context.Context {
	return tenancy.WithTenant(context.Background(), &spec.Tenant{
		ID: id, DisplayName: id, Status: spec.TenantStatusActive, CreatedAt: time.Now().UTC(),
	}, "https://idp.example/realms/"+id, "/realms/"+id)
}

func TestAuthorizeCannotResolveAnotherTenantClient(t *testing.T) {
	clients := memory.NewClientRepository()
	clients.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID, ClientID: "web-app", ClientType: spec.ClientPublic,
		RedirectURIs:            []string{"https://app.example/callback"},
		GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodNone, Scope: "openid",
		IDTokenSignedResponseAlg: spec.SigAlgPS256, FapiProfile: spec.FapiNone,
		CreatedAt: time.Now().UTC(),
	})
	_, err := Authorize(tenantContext("acme"), AuthorizeDeps{
		ClientRepo: clients, RequestStore: memory.NewAuthorizationRequestStore(),
	}, AuthorizeRequestInput{
		ClientID: "web-app", RedirectURI: "https://app.example/callback",
		ResponseType: string(spec.ResponseTypeCode), Scope: "openid",
		CodeChallenge: "challenge", CodeChallengeMethod: string(spec.CodeChallengeMethodS256),
	})
	assertOAuthErrorCode(t, err, "invalid_client")
}

func TestAuthorizationCodeCannotCrossTenantBoundary(t *testing.T) {
	codes := memory.NewAuthorizationCodeStore()
	if err := codes.Save(context.Background(), &spec.AuthorizationCodeRecord{
		Code: "AC1", TenantID: "acme", AuthorizationRequestID: "7856cb4e-7405-4d24-9c04-475cbb13f6f1",
		ClientID: "web-app", Sub: "user", RedirectURI: "https://app.example/callback",
		CodeChallenge: "challenge", CodeChallengeMethod: spec.CodeChallengeMethodS256,
		State: spec.AuthCodeRecordIssued, IssuedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	_, err := ExchangeCodeForToken(tenantContext(spec.DefaultTenantID), ExchangeCodeDeps{
		CodeStore: codes,
	}, ExchangeCodeInput{
		ClientID: "web-app", Code: "AC1", CodeVerifier: "verifier",
		RedirectURI: "https://app.example/callback",
	})
	assertOAuthErrorCode(t, err, "invalid_grant")
}

func TestRefreshTokenCannotCrossTenantBoundary(t *testing.T) {
	clients := memory.NewClientRepository()
	clients.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID, ClientID: "web-app", ClientType: spec.ClientPublic,
		RedirectURIs:            []string{"https://app.example/cb"},
		GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodNone, Scope: "openid",
		IDTokenSignedResponseAlg: spec.SigAlgPS256, FapiProfile: spec.FapiNone,
		CreatedAt: time.Now().UTC(),
	})
	users := memory.NewUserRepository()
	users.Seed(&spec.User{
		Sub: "user", TenantID: spec.DefaultTenantID, PreferredUsername: "alice",
		PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})

	store := memory.NewRefreshTokenStore()
	gen, err := domain.GenerateInitialRefreshToken("web-app", "user", []string{"openid"}, nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	gen.Record.TenantID = "acme" // テナント "acme" 発行
	if err := store.Save(context.Background(), gen.Record); err != nil {
		t.Fatal(err)
	}

	_, err = RefreshTokens(tenantContext(spec.DefaultTenantID), RefreshDeps{
		ClientRepo: clients, UserRepo: users, RefreshStore: store,
	}, RefreshInput{ClientID: "web-app", RefreshToken: gen.Token}, time.Now().UTC())
	assertOAuthErrorCode(t, err, "invalid_grant")
}

func TestDeviceCodeCannotCrossTenantBoundary(t *testing.T) {
	clients := memory.NewClientRepository()
	clients.Seed(&spec.Client{
		TenantID: spec.DefaultTenantID, ClientID: "tv-app", ClientType: spec.ClientPublic,
		RedirectURIs:            []string{"https://tv.example/cb"},
		GrantTypes:              []spec.GrantType{spec.GrantDeviceCode, spec.GrantRefreshToken},
		ResponseTypes:           []spec.ResponseType{},
		TokenEndpointAuthMethod: spec.AuthMethodNone, Scope: "openid",
		IDTokenSignedResponseAlg: spec.SigAlgPS256, FapiProfile: spec.FapiNone,
		CreatedAt: time.Now().UTC(),
	})
	users := memory.NewUserRepository()
	now := time.Now().UTC()
	users.Seed(&spec.User{
		Sub: "user", TenantID: spec.DefaultTenantID, PreferredUsername: "alice",
		PasswordHash: "hash", CreatedAt: now, UpdatedAt: now,
	})

	deviceStore := memory.NewDeviceCodeStore()
	deviceCode := "DEVCODE-acme-1234567890"
	sub := "user"
	authTime := now.Unix()
	rec := &spec.DeviceAuthorization{
		DeviceCodeHash:  domain.HashDeviceCode(deviceCode),
		TenantID:        "acme", // テナント "acme" 発行
		UserCode:        "ABCD-EFGH",
		ClientID:        "tv-app",
		Scopes:          []string{"openid"},
		State:           spec.DeviceFlowApproved,
		Sub:             &sub,
		AuthTime:        &authTime,
		IntervalSeconds: 5,
		IssuedAt:        now,
		ExpiresAt:       now.Add(10 * time.Minute),
	}
	if err := deviceStore.Save(context.Background(), rec); err != nil {
		t.Fatal(err)
	}

	_, err := ExchangeDeviceCode(tenantContext(spec.DefaultTenantID), ExchangeDeviceCodeDeps{
		ClientRepo: clients, UserRepo: users, DeviceCodeStore: deviceStore,
		RefreshStore: memory.NewRefreshTokenStore(), TokenIssuer: &fakeTokenIssuer{},
	}, ExchangeDeviceCodeInput{ClientID: "tv-app", DeviceCode: deviceCode}, now.Add(20*time.Second))
	assertOAuthErrorCode(t, err, "invalid_grant")
}

func assertOAuthErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	oauthErr := &OAuthError{}
	ok := errors.As(err, &oauthErr)
	if !ok || oauthErr.Code != code {
		t.Fatalf("error = %#v, want OAuth code %s", err, code)
	}
}
