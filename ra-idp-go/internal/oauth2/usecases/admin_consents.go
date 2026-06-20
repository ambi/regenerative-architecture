package usecases

// 管理者向け Consent 操作 (List / Get / Revoke)。
// SCL OAuth2 component の admin インターフェース群:
// ListAdminConsents / GetAdminConsent / RevokeAdminConsent。

import (
	"context"
	"errors"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrConsentNotFound = errors.New("consent not found")

type ConsentDeps struct {
	ConsentRepo oauthports.ConsentRepository
	Emit        func(spec.DomainEvent)
}

func ListConsents(ctx context.Context, deps ConsentDeps) ([]*spec.Consent, error) {
	return deps.ConsentRepo.FindAll(ctx, tenancy.TenantID(ctx))
}

func GetConsent(
	ctx context.Context,
	deps ConsentDeps,
	sub, clientID string,
) (*spec.Consent, error) {
	consent, err := deps.ConsentRepo.Find(ctx, tenancy.TenantID(ctx), sub, clientID)
	if err != nil {
		return nil, err
	}
	if consent == nil {
		return nil, ErrConsentNotFound
	}
	return consent, nil
}

func RevokeConsent(
	ctx context.Context,
	deps ConsentDeps,
	actorSub, sub, clientID string,
	now time.Time,
) error {
	if _, err := GetConsent(ctx, deps, sub, clientID); err != nil {
		return err
	}
	if err := deps.ConsentRepo.Revoke(ctx, tenancy.TenantID(ctx), sub, clientID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.ConsentRevokedEvent{
		At: adminNow(now), TenantID: tenancy.TenantID(ctx), ActorSub: actorSub, Sub: sub, ClientID: clientID,
	})
	return nil
}
