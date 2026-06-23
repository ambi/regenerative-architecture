package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// AuthorizationDetailTypeRepository (RFC 9396 / ADR-050)
// =====================================================================

type AuthorizationDetailTypeRepository struct {
	mu    sync.RWMutex
	types map[string]*spec.AuthorizationDetailType // key: tenantKey(tenant_id, type)
}

func NewAuthorizationDetailTypeRepository() *AuthorizationDetailTypeRepository {
	return &AuthorizationDetailTypeRepository{types: map[string]*spec.AuthorizationDetailType{}}
}

// Seed は起動時のサンプル type 投入に使う (テスト・デモ用)。
func (r *AuthorizationDetailTypeRepository) Seed(t *spec.AuthorizationDetailType) {
	_ = r.Save(context.Background(), t)
}

func cloneDetailType(t *spec.AuthorizationDetailType) *spec.AuthorizationDetailType {
	cloned := *t
	cloned.Schema.Rules = slices.Clone(t.Schema.Rules)
	for i := range cloned.Schema.Rules {
		cloned.Schema.Rules[i].Allowed = slices.Clone(t.Schema.Rules[i].Allowed)
	}
	return &cloned
}

func (r *AuthorizationDetailTypeRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.AuthorizationDetailType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.AuthorizationDetailType, 0)
	for _, t := range r.types {
		if t.TenantID == tenantID {
			out = append(out, cloneDetailType(t))
		}
	}
	slices.SortFunc(out, func(a, b *spec.AuthorizationDetailType) int { return strings.Compare(a.Type, b.Type) })
	return out, nil
}

func (r *AuthorizationDetailTypeRepository) FindByType(_ context.Context, tenantID, detailType string) (*spec.AuthorizationDetailType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t := r.types[tenantKey(tenantID, detailType)]
	if t == nil {
		return nil, nil
	}
	return cloneDetailType(t), nil
}

func (r *AuthorizationDetailTypeRepository) Save(_ context.Context, t *spec.AuthorizationDetailType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defaultTenant(&t.TenantID)
	r.types[tenantKey(t.TenantID, t.Type)] = cloneDetailType(t)
	return nil
}

func (r *AuthorizationDetailTypeRepository) Delete(_ context.Context, tenantID, detailType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.types, tenantKey(tenantID, detailType))
	return nil
}
