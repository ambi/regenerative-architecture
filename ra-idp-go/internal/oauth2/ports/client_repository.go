// Package ports: OAuth2 ユースケースが要求する境界。
package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

type OAuth2ClientRepository interface {
	FindByID(ctx context.Context, tenantID, clientID string) (*spec.OAuth2Client, error)
	Save(ctx context.Context, c *spec.OAuth2Client) error
	Delete(ctx context.Context, tenantID, clientID string) error
	FindAll(ctx context.Context, tenantID string) ([]*spec.OAuth2Client, error)
}

type ConsentRepository interface {
	Find(ctx context.Context, tenantID, sub, clientID string) (*spec.Consent, error)
	FindAll(ctx context.Context, tenantID string) ([]*spec.Consent, error)
	Save(ctx context.Context, c *spec.Consent) error
	Revoke(ctx context.Context, tenantID, sub, clientID string) error
	// DeleteAllForSub は ADR-036 の anonymize cascade から呼ばれる。
	// 対象 sub の Consent を物理削除する。
	DeleteAllForSub(ctx context.Context, sub string) error
}
