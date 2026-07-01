package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

// UserRepository は IdentityManagement が所有する User aggregate の永続化境界。
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
