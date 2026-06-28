package ports

import (
	"context"

	"ra-idp-go/internal/shared/spec"
)

type MfaFactorRepository interface {
	ListBySub(ctx context.Context, sub string) ([]*spec.MfaFactor, error)
	Find(ctx context.Context, sub string, factorType spec.MfaFactorType) (*spec.MfaFactor, error)
	Save(ctx context.Context, factor *spec.MfaFactor) error
	Delete(ctx context.Context, sub string, factorType spec.MfaFactorType) error
	// DeleteAllForSub は ADR-036 の anonymize cascade から呼ばれる。
	// 対象 sub の MFA factor をすべて物理削除する。
	DeleteAllForSub(ctx context.Context, sub string) error
}
