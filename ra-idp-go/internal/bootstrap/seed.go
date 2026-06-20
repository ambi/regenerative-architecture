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
	groups authports.GroupRepository,
	hasher authports.PasswordHasher,
) error {
	secretHash := oauthdomain.HashClientSecret(envDefault("DEMO_CLIENT_SECRET", "demo-client-secret"))
	now := time.Now().UTC()
	if err := clients.Save(ctx, &spec.Client{
		TenantID: spec.DefaultTenantID, ClientID: "demo-client",
		ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
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
		Sub: "user_alice", TenantID: spec.DefaultTenantID,
		PreferredUsername: "alice", PasswordHash: hash,
		Email: &email, EmailVerified: true, MfaEnrolled: totpSecret != "",
		Roles:     []string{"admin"},
		Lifecycle: spec.UserLifecycle{Status: spec.UserStatusActive},
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		return err
	}
	if err := passwordHistory.Add(ctx, "user_alice", hash, now); err != nil {
		return err
	}
	if err := seedDemoGroups(ctx, groups, now); err != nil {
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

// seedDemoGroups は固定 ID のデモ用グループ engineering / support を投入し、alice を
// engineering に所属させる。再起動時に重複しないよう ID は固定し、Save は id 上の
// upsert、AddMember は冪等 (no-op on conflict) を利用する。これにより demo.sh で
// グループ由来ロール (engineering → catalog:read) を確認できる。
func seedDemoGroups(ctx context.Context, groups authports.GroupRepository, now time.Time) error {
	engineeringDesc := "プロダクト開発チーム"
	supportDesc := "カスタマーサポートチーム"
	demoGroups := []*spec.Group{
		{
			ID: "group_engineering", TenantID: spec.DefaultTenantID, Name: "engineering",
			Description: &engineeringDesc, Roles: []string{"catalog:read"}, CreatedAt: now,
		},
		{
			ID: "group_support", TenantID: spec.DefaultTenantID, Name: "support",
			Description: &supportDesc, Roles: []string{"invoice:read"}, CreatedAt: now,
		},
	}
	for _, group := range demoGroups {
		if err := groups.Save(ctx, group); err != nil {
			return err
		}
	}
	if _, err := groups.AddMember(ctx, &spec.GroupMember{
		GroupID: "group_engineering", UserSub: "user_alice", AddedAt: now,
	}); err != nil {
		return err
	}
	return nil
}
