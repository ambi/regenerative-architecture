package bootstrap

import (
	"context"
	"errors"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

// seedDemoData は SKIP_DEMO_SEED が空のとき、デモ用クライアントとユーザーを 1 件投入する。
// 既存データを更新する想定で Save を直接呼ぶ。
func seedDemoData(
	ctx context.Context,
	clients oauthports.ClientRepository,
	users oauthports.UserRepository,
	mfaFactors authports.MfaFactorRepository,
	passwordHistory authports.PasswordHistoryRepository,
	hasher authports.PasswordHasher,
) error {
	secretHash := oauthdomain.HashClientSecret(envDefault("DEMO_CLIENT_SECRET", "demo-client-secret"))
	now := time.Now().UTC()
	if err := clients.Save(ctx, &spec.Client{
		ClientID: "demo-client", ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
		RedirectURIs: []string{
			"http://localhost:3000/callback",
			"http://localhost:5173/callback",
		},
		GrantTypes: []spec.GrantType{
			spec.GrantAuthorizationCode, spec.GrantRefreshToken,
			spec.GrantClientCredentials, spec.GrantDeviceCode,
		},
		ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
		TokenEndpointAuthMethod: spec.AuthMethodClientSecretBasic,
		Scope:                   "openid profile email offline_access", IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile: spec.FapiNone, CreatedAt: now,
	}); err != nil {
		return err
	}
	password := envDefault("DEMO_USER_PASSWORD", "demo-password-1234")
	if result := authusecases.ValidatePassword(password); !result.OK {
		return errors.New("DEMO_USER_PASSWORD violates password policy")
	}
	hash, err := hasher.Hash(password)
	if err != nil {
		return err
	}
	email := "alice@example.com"
	totpSecret := envDefault("DEMO_TOTP_SECRET", "")
	if err := users.Save(ctx, &spec.User{
		Sub: "user_alice", PreferredUsername: "alice", PasswordHash: hash,
		Email: &email, EmailVerified: true, MfaEnrolled: totpSecret != "",
		Roles:     []string{"admin"},
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return err
	}
	if err := passwordHistory.Add(ctx, "user_alice", hash, now); err != nil {
		return err
	}
	if totpSecret == "" {
		return nil
	}
	label := "Demo TOTP"
	return mfaFactors.Save(ctx, &spec.MfaFactor{
		Sub: "user_alice", Type: spec.MfaFactorTOTP, Secret: &totpSecret, Label: &label, CreatedAt: now,
	})
}
