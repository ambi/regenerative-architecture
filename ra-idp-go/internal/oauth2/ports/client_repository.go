// Package ports: OAuth2 ユースケースが要求する境界。
package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type OAuth2ClientRepository interface {
	FindByID(ctx context.Context, tenantID, clientID string) (*spec.OAuth2Client, error)
	Save(ctx context.Context, c *spec.OAuth2Client) error
	Delete(ctx context.Context, tenantID, clientID string) error
	FindAll(ctx context.Context, tenantID string) ([]*spec.OAuth2Client, error)
}

type UserRepository interface {
	// FindBySub は ADR-036 の tombstone (`deleted_at != null`) を除外する。
	// 既に削除された user を含めて引きたい場合は FindBySubIncludingDeleted を使う。
	FindBySub(ctx context.Context, sub string) (*spec.User, error)
	// FindBySubIncludingDeleted は tombstone を含めて user を引く。
	// DeleteUser use case の冪等判定や監査経路から呼ばれる。
	FindBySubIncludingDeleted(ctx context.Context, sub string) (*spec.User, error)
	FindByUsername(ctx context.Context, tenantID, username string) (*spec.User, error)
	FindByEmail(ctx context.Context, tenantID, email string) (*spec.User, error)
	FindAll(ctx context.Context, tenantID string) ([]*spec.User, error)
	Save(ctx context.Context, user *spec.User) error
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
