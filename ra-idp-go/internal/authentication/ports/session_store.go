package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type SessionStore interface {
	Save(ctx context.Context, s *spec.LoginSession) error
	Find(ctx context.Context, sessionID string) (*spec.LoginSession, error)
	Delete(ctx context.Context, sessionID string) error
	// DeleteAllForSub は ADR-036 の anonymize cascade から呼ばれる。
	// 対象 sub の LoginSession をすべて物理削除する。
	DeleteAllForSub(ctx context.Context, sub string) error
}
