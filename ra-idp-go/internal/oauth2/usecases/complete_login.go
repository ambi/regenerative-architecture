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
	AMR       []string
	ACR       string
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
	if req.TenantID != tenancy.TenantID(ctx) {
		return nil, NewOAuthError("invalid_request", "未知の authorization request")
	}
	if time.Now().After(req.ExpiresAt) {
		_ = deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowExpired)
		return nil, NewOAuthError("invalid_request", "authorization request 期限切れ")
	}
	if req.State != spec.AuthFlowReceived {
		return nil, NewOAuthError(
			"invalid_request",
			"authorization request は処理済みです。クライアントから認可をやり直してください",
		)
	}

	// received → authentication_pending → authenticated → code_issued の最短経路
	if err := deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowAuthenticationPending); err != nil {
		return nil, err
	}
	if err := deps.RequestStore.UpdateState(ctx, req.ID, spec.AuthFlowAuthenticated); err != nil {
		return nil, err
	}
	authTime := in.AuthTime.UTC().Unix()
	if err := deps.RequestStore.AttachAuthentication(ctx, req.ID, in.Sub, authTime, in.AMR, in.ACR); err != nil {
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
		TenantID:               req.TenantID,
		AuthorizationRequestID: req.ID,
		ClientID:               req.ClientID,
		Sub:                    in.Sub,
		Scopes:                 strings.Fields(req.Scope),
		RedirectURI:            req.RedirectURI,
		CodeChallenge:          req.CodeChallenge,
		CodeChallengeMethod:    req.CodeChallengeMethod,
		Nonce:                  req.Nonce,
		AuthTime:               authTime,
		AMR:                    slices.Clone(in.AMR),
		ACR:                    optional(in.ACR),
		AuthorizationDetails:   req.AuthorizationDetails,
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
