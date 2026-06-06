// /revoke (RFC 7009)
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

type RevokeDeps struct {
	RefreshStore ports.RefreshTokenStore
	Emit         func(spec.DomainEvent)
}

func RevokeToken(ctx context.Context, deps RevokeDeps, token string, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	hash := domain.HashRefreshToken(token)
	rec, err := deps.RefreshStore.FindByHash(ctx, hash)
	if err != nil {
		return err
	}
	if rec == nil {
		// RFC 7009 §2.2: 未知トークンは 200 OK no-op
		return nil
	}
	if err := deps.RefreshStore.RevokeFamily(ctx, rec.FamilyID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.TokenRevoked{At: now, TokenType: "refresh_token", TokenID: rec.ID, Reason: "client_initiated"})
	return nil
}
