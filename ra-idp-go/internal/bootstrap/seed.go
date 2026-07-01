package bootstrap

import (
	"context"
	"errors"
	"time"

	appports "ra-idp-go/internal/application/ports"
	authnports "ra-idp-go/internal/authentication/ports"
	authusecases "ra-idp-go/internal/authentication/usecases"
	idmports "ra-idp-go/internal/identitymanagement/ports"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
)

// seedDemoData は SKIP_DEMO_SEED が空のとき、デモ用クライアントとユーザーを 1 件投入する。
// 既存データを更新する想定で Save を直接呼ぶ。
func seedDemoData(
	ctx context.Context,
	clients oauthports.OAuth2ClientRepository,
	users idmports.UserRepository,
	mfaFactors authnports.MfaFactorRepository,
	passwordHistory authnports.PasswordHistoryRepository,
	groups idmports.GroupRepository,
	authzDetailTypes oauthports.AuthorizationDetailTypeRepository,
	hasher authnports.PasswordHasher,
) error {
	secretHash := oauthdomain.HashClientSecret(envDefault("DEMO_CLIENT_SECRET", "demo-client-secret"))
	now := time.Now().UTC()
	if err := clients.Save(ctx, &spec.OAuth2Client{
		TenantID: spec.DefaultTenantID, ClientID: "demo-client",
		ClientSecretHash: &secretHash, ClientType: spec.ClientConfidential,
		RedirectURIs: []string{
			"http://localhost:3000/callback",
			"http://localhost:5173/callback",
			"http://localhost:8080/callback",
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
	if err := seedFirstPartyPortalClients(ctx, clients, now); err != nil {
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
	if err := seedDemoAuthorizationDetailTypes(ctx, authzDetailTypes, now); err != nil {
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

// seedFirstPartyPortalClients は管理コンソールとアカウントポータルを自分自身の IdP の
// OIDC RP として登録する (ADR-061 / [[wi-66-portals-as-oidc-rp]])。両者は public +
// authorization_code + PKCE のファーストパーティ SPA クライアントで、client secret を
// 持たない (token_endpoint_auth_method = none)。redirect_uri は SPA の `/callback`。
func seedFirstPartyPortalClients(ctx context.Context, clients oauthports.OAuth2ClientRepository, now time.Time) error {
	portals := []struct {
		clientID string
		name     string
		scope    string
	}{
		{"ra-admin-console", "RA Admin Console", "openid profile ra.admin offline_access"},
		{"ra-account-portal", "RA Account Portal", "openid profile ra.account offline_access"},
	}
	for _, p := range portals {
		name := p.name
		if err := clients.Save(ctx, &spec.OAuth2Client{
			TenantID: spec.DefaultTenantID, ClientID: p.clientID,
			ClientName: &name, ClientType: spec.ClientPublic,
			RedirectURIs: []string{
				"http://localhost:3000/callback",
				"http://localhost:5173/callback",
				"http://localhost:8080/callback",
			},
			GrantTypes:              []spec.GrantType{spec.GrantAuthorizationCode, spec.GrantRefreshToken},
			ResponseTypes:           []spec.ResponseType{spec.ResponseTypeCode},
			TokenEndpointAuthMethod: spec.AuthMethodNone,
			Scope:                   p.scope, IDTokenSignedResponseAlg: spec.SigAlgPS256,
			FapiProfile: spec.FapiNone, FirstParty: true, CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// seedDemoApplications は既存の OIDC クライアント / WS-Fed RP を「アプリケーション」として
// カタログに登録する。管理コンソール・アカウントポータル・demo-client・demo WS-Fed RP を
// federated Application として binding 接続し、いずれも user_alice に割り当てる。これにより
// ポータルのアプリ一覧に並び、デモのログイン経路 (割当ゲート) も成立する (wi-69)。
// 管理コンソール / ポータルは first-party のため、割当がなくてもログイン自体は塞がない。
func seedDemoApplications(
	ctx context.Context,
	apps appports.ApplicationRepository,
	assignments appports.AssignmentRepository,
	now time.Time,
) error {
	if apps == nil {
		return nil
	}
	seeds := []struct {
		id        string
		name      string
		launchURL string
		binding   spec.ProtocolBinding
	}{
		{"00000000-0000-4000-8000-000000000101", "RA Admin Console", "/realms/default/admin", spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: "ra-admin-console"}},
		{"00000000-0000-4000-8000-000000000102", "RA Account Portal", "/realms/default/account", spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: "ra-account-portal"}},
		{"00000000-0000-4000-8000-000000000103", "Demo Client", "", spec.ProtocolBinding{Type: spec.ProtocolBindingOIDC, ClientID: "demo-client"}},
		{"00000000-0000-4000-8000-000000000104", "Demo WS-Federation RP", "https://rp.example/wsfed", spec.ProtocolBinding{Type: spec.ProtocolBindingWsFed, Wtrealm: "urn:ra-idp:demo-rp"}},
	}
	for _, s := range seeds {
		if err := apps.Save(ctx, &spec.Application{
			TenantID: spec.DefaultTenantID, ApplicationID: s.id, Name: s.name,
			Kind: spec.ApplicationFederated, Status: spec.ApplicationActive,
			LaunchURL: s.launchURL, Bindings: []spec.ProtocolBinding{s.binding},
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		if assignments == nil {
			continue
		}
		if err := assignments.Save(ctx, &spec.ApplicationAssignment{
			TenantID: spec.DefaultTenantID, ApplicationID: s.id,
			SubjectType: spec.AssignmentSubjectUser, SubjectID: "user_alice",
			Visibility: spec.AssignmentVisible, CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// seedDemoAuthorizationDetailTypes は RFC 9396 のサンプル type を 1 件投入する (ADR-050)。
// payment_initiation は actions を集合包含、creditorAccount を enum、instructedAmount を
// 上限 (単調減少) として扱い、エージェントに「口座 X へ最大 N まで」を束縛させる例。
func seedDemoAuthorizationDetailTypes(ctx context.Context, types oauthports.AuthorizationDetailTypeRepository, now time.Time) error {
	if types == nil {
		return nil
	}
	return types.Save(ctx, &spec.AuthorizationDetailType{
		TenantID:    spec.DefaultTenantID,
		Type:        "payment_initiation",
		Description: "口座から指定上限までの送金開始 (RFC 9396 例)",
		Schema: spec.AuthorizationDetailsSchema{
			Rules: []spec.AuthorizationDetailFieldRule{
				{Name: "actions", Semantics: spec.DetailFieldSet, Required: true, Allowed: []string{"initiate", "status", "cancel"}},
				{Name: "creditorAccount", Semantics: spec.DetailFieldEnum, Required: true},
				{Name: "instructedAmount", Semantics: spec.DetailFieldAtMost, Required: true},
			},
		},
		DisplayTemplate: "口座 {creditorAccount} に対して {actions} を、最大 {instructedAmount} まで",
		State:           spec.DetailTypeEnabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
}

// seedDemoGroups は固定 ID のデモ用グループ engineering / support を投入し、alice を
// engineering に所属させる。再起動時に重複しないよう ID は固定し、Save は id 上の
// upsert、AddMember は冪等 (no-op on conflict) を利用する。これにより demo.sh で
// グループ由来ロール (engineering → catalog:read) を確認できる。
func seedDemoGroups(ctx context.Context, groups idmports.GroupRepository, now time.Time) error {
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
