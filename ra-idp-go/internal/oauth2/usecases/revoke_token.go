// /revoke (RFC 7009)
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

type RevokeDeps struct {
	RefreshStore        ports.RefreshTokenStore
	Introspector        ports.TokenIntrospector
	AccessTokenDenylist ports.AccessTokenDenylist
	Emit                func(spec.DomainEvent)
}

func RevokeToken(ctx context.Context, deps RevokeDeps, clientID, token string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	hash := domain.HashRefreshToken(token)
	rec, err := deps.RefreshStore.FindByHash(ctx, hash)
	if err != nil {
		return err
	}
	if rec == nil {
		return revokeAccessToken(ctx, deps, clientID, token, now)
	}
	if rec.TenantID != tenancy.TenantID(ctx) || rec.ClientID != clientID {
		// RFC 7009 §2.2: 所有者でない要求も 200 OK no-op
		return nil
	}
	if err := deps.RefreshStore.RevokeFamily(ctx, rec.FamilyID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.TokenRevoked{At: now, TenantID: rec.TenantID, TokenType: "refresh_token", TokenID: rec.ID, Reason: "client_initiated"})
	return nil
}

func revokeAccessToken(
	ctx context.Context,
	deps RevokeDeps,
	clientID, token string,
	now time.Time,
) error {
	if deps.Introspector == nil || deps.AccessTokenDenylist == nil {
		return nil
	}
	result, err := deps.Introspector.IntrospectAccessToken(ctx, token)
	if err != nil || !result.Active || result.JTI == "" || result.ClientID != clientID {
		return nil //nolint:nilerr // RFC 7009 requires invalid or unknown tokens to be a successful no-op.
	}
	if err := deps.AccessTokenDenylist.Add(ctx, result.JTI, time.Unix(result.Exp, 0)); err != nil {
		return err
	}
	emit(deps.Emit, &spec.TokenRevoked{
		At: now, TenantID: tenancy.TenantID(ctx), TokenType: "access_token", TokenID: result.JTI, Reason: "client_initiated",
	})
	return nil
}
