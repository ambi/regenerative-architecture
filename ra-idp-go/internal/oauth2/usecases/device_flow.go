// Device Authorization Grant (RFC 8628) のユースケース群:
// /device_authorization (RequestDeviceAuthorization), verify_user_code, exchange_device_code.
package usecases

import (
	"context"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

// =====================================================================
// /device_authorization
// =====================================================================

type DeviceAuthorizationInput struct {
	ClientID string
	Scope    string
}

type DeviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type DeviceAuthorizationDeps struct {
	ClientRepo       ports.ClientRepository
	DeviceCodeStore  ports.DeviceCodeStore
	BaseVerification string // e.g., "https://idp/device"
	Emit             func(spec.DomainEvent)
}

func RequestDeviceAuthorization(ctx context.Context, deps DeviceAuthorizationDeps, in DeviceAuthorizationInput, now time.Time) (*DeviceAuthorizationResponse, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "未知の client_id")
	}
	if !spec.GrantAllowsClientType(spec.GrantDeviceCode, client.ClientType) ||
		!containsGrant(client.GrantTypes, spec.GrantDeviceCode) {
		return nil, NewOAuthError("unauthorized_client", "device_code grant 不可")
	}

	deviceCode, err := domain.GenerateDeviceCode()
	if err != nil {
		return nil, err
	}
	userCode, err := domain.GenerateUserCode()
	if err != nil {
		return nil, err
	}
	scopes := strings.Fields(in.Scope)
	if len(scopes) == 0 {
		scopes = []string{"openid"}
	}
	allowedScopes := strings.Fields(client.Scope)
	for _, scope := range scopes {
		if !slices.Contains(allowedScopes, scope) {
			return nil, NewOAuthError("invalid_scope", "宣言外のスコープが含まれています")
		}
	}
	rec := &spec.DeviceAuthorization{
		TenantID:        tenantID,
		DeviceCodeHash:  domain.HashDeviceCode(deviceCode),
		UserCode:        domain.NormalizeUserCode(userCode),
		ClientID:        client.ClientID,
		Scopes:          scopes,
		State:           spec.DeviceFlowIssued,
		IntervalSeconds: 5,
		IssuedAt:        now,
		ExpiresAt:       now.Add(domain.DeviceCodeTTL),
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	if err := deps.DeviceCodeStore.Save(ctx, rec); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.DeviceAuthorizationRequested{At: now, ClientID: client.ClientID, Scopes: scopes})
	return &DeviceAuthorizationResponse{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		VerificationURI:         deps.BaseVerification,
		VerificationURIComplete: deps.BaseVerification + "?user_code=" + userCode,
		ExpiresIn:               int(domain.DeviceCodeTTL.Seconds()),
		Interval:                5,
	}, nil
}

// =====================================================================
// /device (verify user code + approve/deny)
// =====================================================================

type VerifyUserCodeDeps struct {
	DeviceCodeStore ports.DeviceCodeStore
	Emit            func(spec.DomainEvent)
}

func ApproveUserCode(ctx context.Context, deps VerifyUserCodeDeps, userCode, sub string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	normalized := domain.NormalizeUserCode(userCode)
	rec, err := deps.DeviceCodeStore.FindByUserCode(ctx, normalized)
	if err != nil {
		return err
	}
	if rec == nil {
		return NewOAuthError("invalid_request", "未知の user_code")
	}
	if rec.TenantID != tenancy.TenantID(ctx) {
		return NewOAuthError("invalid_request", "未知の user_code")
	}
	if domain.IsDeviceExpired(rec, now) {
		return NewOAuthError("expired_token", "user_code 期限切れ")
	}
	// 既存状態によらず Issued → UserCodeEntered → Approved に進める (簡略化)。
	if rec.State == spec.DeviceFlowIssued {
		next, err := spec.TransitionDeviceCodeFlow(rec.State, spec.DeviceEventEnterUserCode)
		if err != nil {
			return err
		}
		rec.State = next
	}
	approved, err := spec.TransitionDeviceCodeFlow(rec.State, spec.DeviceEventApprove)
	if err != nil {
		return NewOAuthError("invalid_request", err.Error())
	}
	rec.State = approved
	authTime := now.Unix()
	rec.Sub = &sub
	rec.AuthTime = &authTime
	if err := deps.DeviceCodeStore.Update(ctx, rec); err != nil {
		return err
	}
	emit(deps.Emit, &spec.DeviceAuthorizationApproved{At: now, ClientID: rec.ClientID, Sub: sub})
	return nil
}

func DenyUserCode(ctx context.Context, deps VerifyUserCodeDeps, userCode, sub string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rec, err := deps.DeviceCodeStore.FindByUserCode(ctx, domain.NormalizeUserCode(userCode))
	if err != nil {
		return err
	}
	if rec == nil {
		return NewOAuthError("invalid_request", "未知の user_code")
	}
	if rec.TenantID != tenancy.TenantID(ctx) {
		return NewOAuthError("invalid_request", "未知の user_code")
	}
	if rec.State == spec.DeviceFlowIssued {
		if next, err := spec.TransitionDeviceCodeFlow(rec.State, spec.DeviceEventEnterUserCode); err == nil {
			rec.State = next
		}
	}
	denied, err := spec.TransitionDeviceCodeFlow(rec.State, spec.DeviceEventDeny)
	if err != nil {
		return NewOAuthError("invalid_request", err.Error())
	}
	rec.State = denied
	if err := deps.DeviceCodeStore.Update(ctx, rec); err != nil {
		return err
	}
	emit(deps.Emit, &spec.DeviceAuthorizationDenied{At: now, ClientID: rec.ClientID, Sub: sub})
	return nil
}

// =====================================================================
// /token (device_code grant)
// =====================================================================

type ExchangeDeviceCodeInput struct {
	ClientID     string
	DeviceCode   string
	ProofJKT     string
	ProofX5TS256 string
}

type ExchangeDeviceCodeResult struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int
	Scope        string
}

type ExchangeDeviceCodeDeps struct {
	ClientRepo      ports.ClientRepository
	UserRepo        ports.UserRepository
	DeviceCodeStore ports.DeviceCodeStore
	RefreshStore    ports.RefreshTokenStore
	TokenIssuer     ports.TokenIssuer
	Emit            func(spec.DomainEvent)
}

func ExchangeDeviceCode(ctx context.Context, deps ExchangeDeviceCodeDeps, in ExchangeDeviceCodeInput, now time.Time) (*ExchangeDeviceCodeResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	hash := domain.HashDeviceCode(in.DeviceCode)
	rec, err := deps.DeviceCodeStore.FindByDeviceCodeHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, NewOAuthError("invalid_grant", "未知の device_code")
	}
	tenantID := tenancy.TenantID(ctx)
	if rec.TenantID != tenantID {
		return nil, NewOAuthError("invalid_grant", "未知の device_code")
	}
	if rec.ClientID != in.ClientID {
		return nil, NewOAuthError("invalid_grant", "device_code がクライアントに紐づきません")
	}
	if domain.IsDeviceExpired(rec, now) {
		return nil, NewOAuthError("expired_token", "device_code 期限切れ")
	}
	switch rec.State {
	case spec.DeviceFlowIssued, spec.DeviceFlowUserCodeEntered:
		if rec.LastPolledAt != nil && now.Sub(*rec.LastPolledAt) < time.Duration(rec.IntervalSeconds)*time.Second {
			rec.IntervalSeconds += spec.DefaultDeviceCodePolling().SlowDownIncrementSeconds
			t := now
			rec.LastPolledAt = &t
			if err := deps.DeviceCodeStore.Update(ctx, rec); err != nil {
				return nil, err
			}
			return nil, NewOAuthError("slow_down", "polling interval が短すぎます")
		}
		t := now
		rec.LastPolledAt = &t
		if err := deps.DeviceCodeStore.Update(ctx, rec); err != nil {
			return nil, err
		}
		return nil, NewOAuthError("authorization_pending", "ユーザー承認待ち")
	case spec.DeviceFlowDenied:
		return nil, NewOAuthError("access_denied", "ユーザー拒否")
	case spec.DeviceFlowExchanged:
		return nil, NewOAuthError("invalid_grant", "device_code はすでに交換済み")
	}
	if rec.State != spec.DeviceFlowApproved {
		return nil, NewOAuthError("invalid_grant", "device_code 状態不正: "+string(rec.State))
	}
	exchanged, err := deps.DeviceCodeStore.Exchange(ctx, hash)
	if err != nil {
		return nil, err
	}
	if exchanged == nil {
		return nil, NewOAuthError("invalid_grant", "device_code は並行リクエストにより交換済みです")
	}
	rec = exchanged

	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil || rec.Sub == nil {
		return nil, NewOAuthError("server_error", "client or sub missing")
	}
	user, err := deps.UserRepo.FindBySub(ctx, *rec.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, NewOAuthError("server_error", "user missing")
	}
	if user.TenantID != tenantID {
		return nil, NewOAuthError("invalid_grant", "未知の device_code")
	}
	if user.DisabledAt != nil {
		return nil, NewOAuthError("invalid_grant", "ユーザーは無効化されています")
	}

	var sc *spec.SenderConstraint
	if in.ProofJKT != "" {
		sc = &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: in.ProofJKT}
	} else if in.ProofX5TS256 != "" {
		sc = &spec.SenderConstraint{Type: spec.SenderConstraintMTLS, X5TS256: in.ProofX5TS256}
	}

	access, jti, err := deps.TokenIssuer.SignAccessToken(ctx, ports.AccessTokenInput{
		Client: client, Sub: user.Sub, Scopes: rec.Scopes, SenderConstraint: sc, AuthTime: *rec.AuthTime,
	})
	if err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.AccessTokenIssued{At: now, JTI: jti, ClientID: client.ClientID, Sub: user.Sub, Scopes: rec.Scopes, SenderConstraint: senderConstraintTag(sc)})

	idTok, err := deps.TokenIssuer.SignIDToken(ctx, ports.IDTokenInput{
		Client: client, User: user, Scopes: rec.Scopes, AuthTime: *rec.AuthTime, AtHashFor: access,
	})
	if err != nil {
		return nil, err
	}

	refresh, err := domain.GenerateInitialRefreshToken(client.ClientID, user.Sub, rec.Scopes, sc, now)
	if err != nil {
		return nil, err
	}
	refresh.Record.TenantID = tenantID
	if err := deps.RefreshStore.Save(ctx, refresh.Record); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.RefreshTokenIssued{At: now, TokenID: refresh.Record.ID, FamilyID: refresh.Record.FamilyID, ClientID: client.ClientID, Sub: user.Sub})
	fam := refresh.Record.FamilyID
	rec.IssuedFamilyID = &fam
	_ = deps.DeviceCodeStore.Update(ctx, rec)

	tokenType := "Bearer"
	if sc != nil && sc.Type == spec.SenderConstraintDPoP {
		tokenType = "DPoP"
	}
	return &ExchangeDeviceCodeResult{
		AccessToken:  access,
		RefreshToken: refresh.Token,
		IDToken:      idTok,
		TokenType:    tokenType,
		ExpiresIn:    deps.TokenIssuer.AccessTokenTTLSeconds(),
		Scope:        strings.Join(rec.Scopes, " "),
	}, nil
}

func containsGrant(grants []spec.GrantType, g spec.GrantType) bool {
	for _, x := range grants {
		if x == g {
			return true
		}
	}
	return false
}
