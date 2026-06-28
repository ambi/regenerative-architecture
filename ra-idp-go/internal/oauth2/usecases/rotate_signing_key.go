// /rotate (内部運用 — JWKS 鍵回転)
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
)

type RotateSigningKeyDeps struct {
	KeyStore ports.KeyStore
	Emit     func(spec.DomainEvent)
}

func RotateSigningKey(ctx context.Context, deps RotateSigningKeyDeps, now time.Time) (*ports.SigningKey, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	prev, _ := deps.KeyStore.GetActiveKey(ctx)
	next, err := deps.KeyStore.Rotate(ctx)
	if err != nil {
		return nil, err
	}
	prevKID := ""
	if prev != nil {
		prevKID = prev.Kid
	}
	emit(deps.Emit, &spec.SigningKeyRotated{At: now, NewKID: next.Kid, PreviousKID: prevKID})
	return next, nil
}
