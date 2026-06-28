package memory

import (
	"context"
	"strings"
	"sync"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// MfaFactorRepository (Authentication)
// =====================================================================

type MfaFactorRepository struct {
	mu      sync.RWMutex
	factors map[string]*spec.MfaFactor
}

func NewMfaFactorRepository() *MfaFactorRepository {
	return &MfaFactorRepository{factors: map[string]*spec.MfaFactor{}}
}

func (r *MfaFactorRepository) ListBySub(_ context.Context, sub string) ([]*spec.MfaFactor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []*spec.MfaFactor{}
	for _, factor := range r.factors {
		if factor.Sub == sub {
			out = append(out, cloneMfaFactor(factor))
		}
	}
	return out, nil
}

func (r *MfaFactorRepository) Find(
	_ context.Context,
	sub string,
	factorType spec.MfaFactorType,
) (*spec.MfaFactor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneMfaFactor(r.factors[mfaFactorKey(sub, factorType)]), nil
}

func (r *MfaFactorRepository) Save(_ context.Context, factor *spec.MfaFactor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factors[mfaFactorKey(factor.Sub, factor.Type)] = cloneMfaFactor(factor)
	return nil
}

func (r *MfaFactorRepository) Delete(_ context.Context, sub string, factorType spec.MfaFactorType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factors, mfaFactorKey(sub, factorType))
	return nil
}

func (r *MfaFactorRepository) DeleteAllForSub(_ context.Context, sub string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := sub + "|"
	for key := range r.factors {
		if strings.HasPrefix(key, prefix) {
			delete(r.factors, key)
		}
	}
	return nil
}

func mfaFactorKey(sub string, factorType spec.MfaFactorType) string {
	return sub + "|" + string(factorType)
}

func cloneMfaFactor(factor *spec.MfaFactor) *spec.MfaFactor {
	if factor == nil {
		return nil
	}
	out := *factor
	if factor.Secret != nil {
		secret := *factor.Secret
		out.Secret = &secret
	}
	if factor.Label != nil {
		label := *factor.Label
		out.Label = &label
	}
	if factor.LastUsedAt != nil {
		lastUsedAt := *factor.LastUsedAt
		out.LastUsedAt = &lastUsedAt
	}
	return &out
}
