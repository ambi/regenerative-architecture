package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type RefreshTokenStore interface {
	FindByHash(ctx context.Context, hash string) (*spec.RefreshTokenRecord, error)
	Save(ctx context.Context, rec *spec.RefreshTokenRecord) error
	// Rotate は parentId を rotated にしつつ新レコードを atomic に保存。
	Rotate(ctx context.Context, parentID string, newRec *spec.RefreshTokenRecord) (*spec.RefreshTokenRecord, error)
	RevokeFamily(ctx context.Context, familyID string) error
	// DeleteAllForSub は ADR-036 の anonymize cascade から呼ばれる。
	// 対象 sub の RefreshToken を物理削除する。
	DeleteAllForSub(ctx context.Context, sub string) error
}
