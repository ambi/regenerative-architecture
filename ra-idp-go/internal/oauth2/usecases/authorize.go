// Package usecases: OAuth2 ユースケース層。
//
// authorize / login / token のエンドポイント間で共有される最小限の振る舞いを
// 純粋に表現する。HTTP 依存は持たない。
package usecases

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

// =====================================================================
// /authorize 受付
// =====================================================================

type AuthorizeRequestInput struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	StateParam          string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	Prompt              string
	MaxAge              *int
	ParUsed             bool
	ParRequestURI       string
}

type AuthorizeRequestOutput struct {
	Request *spec.AuthorizationRequest
	Client  *spec.Client
}

type AuthorizeDeps struct {
	ClientRepo   ports.ClientRepository
	RequestStore ports.AuthorizationRequestStore
}

// OAuthError は redirect 経由で返すべき OAuth2 規定のエラー。
// HTTP 層が code/description を redirect_uri クエリに展開する。
type OAuthError struct {
	Code        string
	Description string
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Description)
}

func NewOAuthError(code, description string) *OAuthError {
	return &OAuthError{Code: code, Description: description}
}

// Authorize は /authorize のリクエスト検証と保存を行う。
// 認可コードフローのみ対応、PKCE S256 必須（デモ簡略化、ロードマップ Phase 1 で
// public/FAPI required・confidential recommended に階段化予定）。
func Authorize(ctx context.Context, deps AuthorizeDeps, in AuthorizeRequestInput) (*AuthorizeRequestOutput, error) {
	if in.ClientID == "" {
		return nil, NewOAuthError("invalid_request", "client_id が必要です")
	}
	if in.RedirectURI == "" {
		return nil, NewOAuthError("invalid_request", "redirect_uri が必要です")
	}
	if in.ResponseType != "code" {
		return nil, NewOAuthError("unsupported_response_type", "code のみサポート")
	}
	if in.CodeChallenge == "" {
		return nil, NewOAuthError("invalid_request", "code_challenge が必要です")
	}
	if in.CodeChallengeMethod != "S256" {
		return nil, NewOAuthError("invalid_request", "code_challenge_method は S256 のみ")
	}

	client, err := deps.ClientRepo.FindByID(ctx, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "未知の client_id")
	}
	if !slices.Contains(client.RedirectURIs, in.RedirectURI) {
		return nil, NewOAuthError("invalid_request", "redirect_uri が登録済み URI ではありません")
	}
	if !slices.Contains(client.GrantTypes, spec.GrantAuthorizationCode) {
		return nil, NewOAuthError("unauthorized_client", "authorization_code grant が宣言されていません")
	}
	requestedScopes := strings.Fields(defaultScope(in.Scope))
	allowedScopes := strings.Fields(client.Scope)
	for _, scope := range requestedScopes {
		if !slices.Contains(allowedScopes, scope) {
			return nil, NewOAuthError("invalid_scope", "宣言外のスコープが含まれています")
		}
	}
	if client.RequirePushedAuthorizationRequests && !in.ParUsed {
		return nil, NewOAuthError("invalid_request", "このクライアントは PAR が必須です")
	}
	if in.Prompt != "" && in.Prompt != "none" && in.Prompt != "login" && in.Prompt != "consent" {
		return nil, NewOAuthError("invalid_request", "未対応の prompt です")
	}

	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	req := &spec.AuthorizationRequest{
		ID:                  id,
		State:               spec.AuthFlowReceived,
		ClientID:            in.ClientID,
		RedirectURI:         in.RedirectURI,
		ResponseType:        spec.ResponseTypeCode,
		Scope:               strings.Join(requestedScopes, " "),
		StateParam:          optional(in.StateParam),
		Nonce:               optional(in.Nonce),
		CodeChallenge:       in.CodeChallenge,
		CodeChallengeMethod: spec.CodeChallengeMethodS256,
		Prompt:              optional(in.Prompt),
		MaxAge:              in.MaxAge,
		ParRequestURI:       optional(in.ParRequestURI),
		CreatedAt:           now,
		ExpiresAt:           now.Add(10 * time.Minute),
	}
	if err := req.Validate(); err != nil {
		return nil, NewOAuthError("invalid_request", err.Error())
	}
	if err := deps.RequestStore.Save(ctx, req); err != nil {
		return nil, err
	}
	return &AuthorizeRequestOutput{Request: req, Client: client}, nil
}

func defaultScope(s string) string {
	if strings.TrimSpace(s) == "" {
		return "openid"
	}
	return s
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// =====================================================================
// /login (POST) → 認証完了 → authorization code 発行
// =====================================================================

type CompleteLoginDeps struct {
	RequestStore ports.AuthorizationRequestStore
	CodeStore    ports.AuthorizationCodeStore
}

type CompleteLoginInput struct {
	RequestID string
	Sub       string
	AuthTime  time.Time
}

type CompleteLoginOutput struct {
	Request *spec.AuthorizationRequest
	Code    *spec.AuthorizationCodeRecord
}

// CompleteLogin は認証・同意確認済みのリクエストに対して状態機械を回し、
// 認可コードを発行する。同意の要否判断と保存は HTTP 継続処理が先に行う。
func CompleteLogin(ctx context.Context, deps CompleteLoginDeps, in CompleteLoginInput) (*CompleteLoginOutput, error) {
	req, err := deps.RequestStore.Find(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, NewOAuthError("invalid_request", "未知の authorization request")
	}
	if time.Now().After(req.ExpiresAt) {
		_ = deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowExpired)
		return nil, NewOAuthError("invalid_request", "authorization request 期限切れ")
	}

	// received → authentication_pending → authenticated → code_issued の最短経路
	if err := deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowAuthenticationPending); err != nil {
		return nil, err
	}
	if err := deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowAuthenticated); err != nil {
		return nil, err
	}
	authTime := in.AuthTime.UTC().Unix()
	if err := deps.RequestStore.AttachSubject(ctx, req.ID, in.Sub, authTime); err != nil {
		return nil, err
	}
	if err := deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowCodeIssued); err != nil {
		return nil, err
	}

	codeValue, err := generateOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	record := &spec.AuthorizationCodeRecord{
		Code:                   codeValue,
		AuthorizationRequestID: req.ID,
		ClientID:               req.ClientID,
		Sub:                    in.Sub,
		Scopes:                 strings.Fields(req.Scope),
		RedirectURI:            req.RedirectURI,
		CodeChallenge:          req.CodeChallenge,
		CodeChallengeMethod:    req.CodeChallengeMethod,
		Nonce:                  req.Nonce,
		AuthTime:               authTime,
		State:                  spec.AuthCodeRecordIssued,
		IssuedAt:               now,
		ExpiresAt:              now.Add(60 * time.Second), // RFC 9700 §4.10
	}
	if err := record.Validate(); err != nil {
		return nil, err
	}
	if err := deps.CodeStore.Save(ctx, record); err != nil {
		return nil, err
	}
	return &CompleteLoginOutput{Request: req, Code: record}, nil
}

// =====================================================================
// /token (authorization_code grant) → access_token + id_token
// =====================================================================

type ExchangeCodeDeps struct {
	ClientRepo   ports.ClientRepository
	UserRepo     ports.UserRepository
	RequestStore ports.AuthorizationRequestStore
	CodeStore    ports.AuthorizationCodeStore
	RefreshStore ports.RefreshTokenStore
	TokenIssuer  ports.TokenIssuer
	Emit         func(spec.DomainEvent)
}

type ExchangeCodeInput struct {
	ClientID     string
	Code         string
	CodeVerifier string
	RedirectURI  string
	DpopJKT      string
}

type ExchangeCodeOutput struct {
	AccessToken  string
	IDToken      string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	Scope        string
}

func ExchangeCodeForToken(ctx context.Context, deps ExchangeCodeDeps, in ExchangeCodeInput) (*ExchangeCodeOutput, error) {
	if in.Code == "" {
		return nil, NewOAuthError("invalid_request", "code が必要です")
	}
	if in.CodeVerifier == "" {
		return nil, NewOAuthError("invalid_request", "code_verifier が必要です")
	}
	if in.RedirectURI == "" {
		return nil, NewOAuthError("invalid_request", "redirect_uri が必要です")
	}

	rec, err := deps.CodeStore.Find(ctx, in.Code)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, NewOAuthError("invalid_grant", "code が無効です")
	}
	now := time.Now().UTC()
	if rec.State != spec.AuthCodeRecordIssued || !now.Before(rec.ExpiresAt) {
		if rec.IssuedFamilyID != nil && deps.RefreshStore != nil {
			_ = deps.RefreshStore.RevokeFamily(ctx, *rec.IssuedFamilyID)
		}
		return nil, NewOAuthError("invalid_grant", "code が使用済みまたは期限切れ")
	}
	if rec.ClientID != in.ClientID {
		return nil, NewOAuthError("invalid_grant", "code がクライアントに紐づかない")
	}
	if rec.RedirectURI != in.RedirectURI {
		return nil, NewOAuthError("invalid_grant", "redirect_uri が一致しない")
	}
	if !domain.VerifyPKCES256(in.CodeVerifier, rec.CodeChallenge) {
		return nil, NewOAuthError("invalid_grant", "PKCE 検証失敗")
	}

	client, err := deps.ClientRepo.FindByID(ctx, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "未知の client_id")
	}
	user, err := deps.UserRepo.FindBySub(ctx, rec.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user attached to code no longer exists")
	}
	redeemed, err := deps.CodeStore.Redeem(ctx, in.Code, now)
	if err != nil {
		return nil, err
	}
	if redeemed == nil {
		if rec.IssuedFamilyID != nil && deps.RefreshStore != nil {
			_ = deps.RefreshStore.RevokeFamily(ctx, *rec.IssuedFamilyID)
		}
		return nil, NewOAuthError("invalid_grant", "code は並行リクエストにより使用済みです")
	}
	rec = redeemed

	var sc *spec.SenderConstraint
	if in.DpopJKT != "" {
		sc = &spec.SenderConstraint{Type: spec.SenderConstraintDPoP, JKT: in.DpopJKT}
	}

	access, jti, err := deps.TokenIssuer.SignAccessToken(ctx, ports.AccessTokenInput{
		Client:           client,
		Sub:              user.Sub,
		Scopes:           rec.Scopes,
		SenderConstraint: sc,
		AuthTime:         rec.AuthTime,
	})
	if err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.AccessTokenIssued{At: now, JTI: jti, ClientID: client.ClientID, Sub: user.Sub, Scopes: rec.Scopes, SenderConstraint: senderConstraintTag(sc)})
	emit(deps.Emit, &spec.AuthorizationCodeRedeemed{At: now, ClientID: client.ClientID, Sub: user.Sub})

	var idToken string
	if slices.Contains(rec.Scopes, "openid") {
		idToken, err = deps.TokenIssuer.SignIDToken(ctx, ports.IDTokenInput{
			Client:    client,
			User:      user,
			Scopes:    rec.Scopes,
			Nonce:     rec.Nonce,
			AuthTime:  rec.AuthTime,
			AtHashFor: access,
		})
		if err != nil {
			return nil, err
		}
	}

	var refreshToken string
	if deps.RefreshStore != nil && slices.Contains(rec.Scopes, "offline_access") {
		gen, err := domain.GenerateInitialRefreshToken(client.ClientID, user.Sub, rec.Scopes, sc, now)
		if err != nil {
			return nil, err
		}
		if err := deps.RefreshStore.Save(ctx, gen.Record); err != nil {
			return nil, err
		}
		emit(deps.Emit, &spec.RefreshTokenIssued{At: now, TokenID: gen.Record.ID, FamilyID: gen.Record.FamilyID, ClientID: client.ClientID, Sub: user.Sub})
		if err := deps.CodeStore.LinkFamily(ctx, rec.Code, gen.Record.FamilyID); err != nil {
			return nil, err
		}
		refreshToken = gen.Token
	}

	if deps.RequestStore != nil {
		_ = deps.RequestStore.UpdateState(ctx, rec.AuthorizationRequestID, spec.AuthFlowExchanged)
	}

	tokenType := "Bearer"
	if sc != nil {
		tokenType = "DPoP"
	}
	return &ExchangeCodeOutput{
		AccessToken:  access,
		IDToken:      idToken,
		RefreshToken: refreshToken,
		TokenType:    tokenType,
		ExpiresIn:    deps.TokenIssuer.AccessTokenTTLSeconds(),
		Scope:        strings.Join(rec.Scopes, " "),
	}, nil
}
