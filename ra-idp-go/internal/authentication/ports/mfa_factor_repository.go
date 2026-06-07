package ports

import (
	"context"

	"ra-idp-go/internal/spec"
)

type MfaFactorRepository interface {
	ListBySub(ctx context.Context, sub string) ([]*spec.MfaFactor, error)
	Find(ctx context.Context, sub string, factorType spec.MfaFactorType) (*spec.MfaFactor, error)
	Save(ctx context.Context, factor *spec.MfaFactor) error
	Delete(ctx context.Context, sub string, factorType spec.MfaFactorType) error
}
