package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
	"ra-idp-go/internal/spec"
)

type deviceFixture struct {
	deps        ExchangeDeviceCodeDeps
	requestDeps DeviceAuthorizationDeps
	verifyDeps  VerifyUserCodeDeps
}

func newDeviceFixture() deviceFixture {
	clientRepo := memory.NewClientRepository()
	userRepo := memory.NewUserRepository()
	deviceStore := memory.NewDeviceCodeStore()
	refreshStore := memory.NewRefreshTokenStore()
	now := time.Now().UTC()
	clientRepo.Seed(&spec.Client{
		ClientID: "device-client", ClientType: spec.ClientPublic,
		RedirectURIs: []string{"https://device.example/cb"},
		GrantTypes:   []spec.GrantType{spec.GrantDeviceCode, spec.GrantRefreshToken},
		ResponseTypes: []spec.ResponseType{
			spec.ResponseTypeCode,
		},
		TokenEndpointAuthMethod:  spec.AuthMethodNone,
		Scope:                    "openid profile",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                now,
	})
	userRepo.Seed(&spec.User{
		Sub: "user", PreferredUsername: "alice", PasswordHash: "hash",
		CreatedAt: now, UpdatedAt: now,
	})
	return deviceFixture{
		requestDeps: DeviceAuthorizationDeps{
			ClientRepo: clientRepo, DeviceCodeStore: deviceStore,
			BaseVerification: "https://idp.example/device",
		},
		verifyDeps: VerifyUserCodeDeps{DeviceCodeStore: deviceStore},
		deps: ExchangeDeviceCodeDeps{
			ClientRepo: clientRepo, UserRepo: userRepo, DeviceCodeStore: deviceStore,
			RefreshStore: refreshStore, TokenIssuer: &fakeTokenIssuer{},
		},
	}
}

func TestDeviceFlowPollingAndReplay(t *testing.T) {
	f := newDeviceFixture()
	t0 := time.Now().UTC()
	auth, err := RequestDeviceAuthorization(
		context.Background(),
		f.requestDeps,
		DeviceAuthorizationInput{ClientID: "device-client", Scope: "openid"},
		t0,
	)
	if err != nil {
		t.Fatal(err)
	}
	input := ExchangeDeviceCodeInput{ClientID: "device-client", DeviceCode: auth.DeviceCode}
	if _, err := ExchangeDeviceCode(context.Background(), f.deps, input, t0); oauthErrorCode(err) != "authorization_pending" {
		t.Fatalf("first poll: %v", err)
	}
	if _, err := ExchangeDeviceCode(context.Background(), f.deps, input, t0.Add(time.Second)); oauthErrorCode(err) != "slow_down" {
		t.Fatalf("fast poll: %v", err)
	}
	if err := ApproveUserCode(
		context.Background(), f.verifyDeps, auth.UserCode, "user", t0.Add(2*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	out, err := ExchangeDeviceCode(context.Background(), f.deps, input, t0.Add(11*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.IDToken == "" || out.RefreshToken == "" {
		t.Fatal("device exchange did not issue tokens")
	}
	if _, err := ExchangeDeviceCode(context.Background(), f.deps, input, t0.Add(20*time.Second)); oauthErrorCode(err) != "invalid_grant" {
		t.Fatalf("replay: %v", err)
	}
}

func TestDeviceAuthorizationRejectsUndeclaredScope(t *testing.T) {
	f := newDeviceFixture()
	_, err := RequestDeviceAuthorization(
		context.Background(),
		f.requestDeps,
		DeviceAuthorizationInput{ClientID: "device-client", Scope: "openid admin"},
		time.Now(),
	)
	if oauthErrorCode(err) != "invalid_scope" {
		t.Fatalf("got %v", err)
	}
}

func oauthErrorCode(err error) string {
	var oauthErr *OAuthError
	if errors.As(err, &oauthErr) {
		return oauthErr.Code
	}
	return ""
}
