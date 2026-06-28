// リフレッシュトークンによる再発行。ADR-004 ローテーション + ファミリー失効。
package usecases

import (
	"context"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

type RefreshInput struct {
	ClientID     string
	RefreshToken string
	ProofJKT     string
	ProofX5TS256 string
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
	Scope        string
}

type RefreshDeps struct {
	ClientRepo   ports.OAuth2ClientRepository
	UserRepo     ports.UserRepository
	RefreshStore ports.RefreshTokenStore
	TokenIssuer  ports.TokenIssuer
	Authorizer   ports.Authorizer
	Emit         func(spec.DomainEvent)
}

func RefreshTokens(ctx context.Context, deps RefreshDeps, in RefreshInput, now time.Time) (*RefreshResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "クライアント認証に失敗")
	}
	hash := domain.HashRefreshToken(in.RefreshToken)
	record, err := deps.RefreshStore.FindByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークンが無効")
	}
	if record.TenantID != tenantID {
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークンが無効")
	}
	if record.ClientID != client.ClientID {
		_ = deps.RefreshStore.RevokeFamily(ctx, record.FamilyID)
		emit(deps.Emit, &spec.RefreshTokenReuseDetected{At: now, TenantID: tenantID, FamilyID: record.FamilyID, TokenID: record.ID, ClientID: client.ClientID})
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークンの所有者が一致しません")
	}
	if domain.IsRefreshTokenReplay(record) {
		_ = deps.RefreshStore.RevokeFamily(ctx, record.FamilyID)
		emit(deps.Emit, &spec.RefreshTokenReuseDetected{At: now, TenantID: tenantID, FamilyID: record.FamilyID, TokenID: record.ID, ClientID: client.ClientID})
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークンはすでに使用されています")
	}
	if domain.IsRefreshTokenAbsoluteExpired(record, now) {
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークン絶対期限切れ")
	}
	user, err := deps.UserRepo.FindBySub(ctx, record.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil {
		_ = deps.RefreshStore.RevokeFamily(ctx, record.FamilyID)
		return nil, NewOAuthError("invalid_grant", "ユーザーは利用できません")
	}
	if user.TenantID != tenantID {
		return nil, NewOAuthError("invalid_grant", "リフレッシュトークンが無効")
	}
	if !user.IsActive() {
		_ = deps.RefreshStore.RevokeFamily(ctx, record.FamilyID)
		return nil, NewOAuthError("invalid_grant", "ユーザーは無効化されています")
	}

	d, err := evaluateRefreshPolicy(ctx, deps.Authorizer, client, record, in, now)
	if err != nil {
		return nil, err
	}
	if !d.Permit {
		return nil, NewOAuthError("invalid_grant", "リフレッシュ拒否: "+strings.Join(d.Reasons, ", "))
	}

	newTok, err := domain.RotateRefreshToken(record, now)
	if err != nil {
		return nil, err
	}
	rotated, err := deps.RefreshStore.Rotate(ctx, record.ID, newTok.Record)
	if err != nil {
		return nil, err
	}
	if rotated == nil {
		_ = deps.RefreshStore.RevokeFamily(ctx, record.FamilyID)
		return nil, NewOAuthError("invalid_grant", "並行リフレッシュにより失効")
	}
	emit(deps.Emit, &spec.RefreshTokenRotated{At: now, TenantID: tenantID, OldTokenID: record.ID, NewTokenID: newTok.Record.ID, FamilyID: record.FamilyID})

	access, jti, err := deps.TokenIssuer.SignAccessToken(ctx, ports.AccessTokenInput{
		Client:           client,
		Sub:              record.Sub,
		Scopes:           record.Scopes,
		SenderConstraint: record.SenderConstraint,
		AuthTime:         now.Unix(),
	})
	if err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.AccessTokenIssued{At: now, TenantID: tenantID, JTI: jti, ClientID: client.ClientID, Sub: record.Sub, Scopes: record.Scopes, SenderConstraint: senderConstraintTag(record.SenderConstraint)})

	tokenType := "Bearer"
	if record.SenderConstraint != nil && record.SenderConstraint.Type == spec.SenderConstraintDPoP {
		tokenType = "DPoP"
	}
	return &RefreshResult{
		AccessToken:  access,
		RefreshToken: newTok.Token,
		TokenType:    tokenType,
		ExpiresIn:    deps.TokenIssuer.AccessTokenTTLSeconds(),
		Scope:        strings.Join(record.Scopes, " "),
	}, nil
}

func evaluateRefreshPolicy(
	ctx context.Context,
	authorizer ports.Authorizer,
	client *spec.OAuth2Client,
	record *spec.RefreshTokenRecord,
	in RefreshInput,
	now time.Time,
) (spec.AuthZResponse, error) {
	props := spec.AuthZResourceProps{
		Revoked:           record.Revoked,
		Rotated:           record.Rotated,
		AbsoluteExpiresAt: record.AbsoluteExpiresAt,
		SenderConstraint:  authZSenderConstraint(record.SenderConstraint),
	}
	var pop *spec.AuthZProofOfPossession
	if record.SenderConstraint != nil {
		valid := false
		switch record.SenderConstraint.Type {
		case spec.SenderConstraintDPoP:
			valid = record.SenderConstraint.JKT == in.ProofJKT
		case spec.SenderConstraintMTLS:
			valid = record.SenderConstraint.X5TS256 == in.ProofX5TS256
		}
		pop = &spec.AuthZProofOfPossession{Valid: valid}
	}
	req := spec.AuthZRequest{
		Subject:  spec.AuthZSubject{Type: "Client", ID: client.ClientID, Properties: spec.AuthZSubjectProps{GrantTypes: client.GrantTypes}},
		Action:   spec.ActionTokenGrantRefresh,
		Resource: spec.AuthZResource{Type: "RefreshToken", Properties: props},
		Context:  spec.AuthZContext{ProofOfPossession: pop, Now: now},
	}
	if authorizer == nil {
		return spec.Evaluate(req), nil
	}
	return authorizer.Authorize(ctx, req)
}

func authZSenderConstraint(sc *spec.SenderConstraint) *spec.AuthZSenderConstraint {
	if sc == nil {
		return nil
	}
	return &spec.AuthZSenderConstraint{Type: sc.Type}
}
