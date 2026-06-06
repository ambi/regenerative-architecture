package usecases

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

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
