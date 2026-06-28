// Package usecases: OAuth2 ユースケース層。
//
// authorize / login / token のエンドポイント間で共有される最小限の振る舞いを
// 純粋に表現する。HTTP 依存は持たない。
package usecases

import (
	"context"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

// =====================================================================
// /authorize 受付
// =====================================================================

type AuthorizeRequestInput struct {
	ClientID             string
	RedirectURI          string
	ResponseType         string
	Scope                string
	StateParam           string
	Nonce                string
	CodeChallenge        string
	CodeChallengeMethod  string
	Prompt               string
	MaxAge               *int
	ACRValues            string
	ParUsed              bool
	ParRequestURI        string
	AuthorizationDetails []spec.AuthorizationDetail
}

type AuthorizeRequestOutput struct {
	Request *spec.AuthorizationRequest
	Client  *spec.OAuth2Client
}

type AuthorizeDeps struct {
	ClientRepo          ports.OAuth2ClientRepository
	RequestStore        ports.AuthorizationRequestStore
	AuthzDetailTypeRepo ports.AuthorizationDetailTypeRepository
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

	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
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

	// RFC 9396 authorization_details: 登録済み type に対し fail-closed 検証 (ADR-050)。
	if err := ValidateAuthorizationDetails(ctx, deps.AuthzDetailTypeRepo, in.AuthorizationDetails); err != nil {
		return nil, err
	}

	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	req := &spec.AuthorizationRequest{
		TenantID:             tenantID,
		ID:                   id,
		State:                spec.AuthFlowReceived,
		ClientID:             in.ClientID,
		RedirectURI:          in.RedirectURI,
		ResponseType:         spec.ResponseTypeCode,
		Scope:                strings.Join(requestedScopes, " "),
		StateParam:           optional(in.StateParam),
		Nonce:                optional(in.Nonce),
		CodeChallenge:        in.CodeChallenge,
		CodeChallengeMethod:  spec.CodeChallengeMethodS256,
		Prompt:               optional(in.Prompt),
		MaxAge:               in.MaxAge,
		ACRValues:            optional(in.ACRValues),
		ParRequestURI:        optional(in.ParRequestURI),
		AuthorizationDetails: in.AuthorizationDetails,
		CreatedAt:            now,
		ExpiresAt:            now.Add(10 * time.Minute),
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
