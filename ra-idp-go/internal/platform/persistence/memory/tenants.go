package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// TenantRepository (Tenancy)
// =====================================================================

type TenantRepository struct {
	mu      sync.RWMutex
	tenants map[string]*spec.Tenant
}

func NewTenantRepository() *TenantRepository {
	return &TenantRepository{tenants: map[string]*spec.Tenant{}}
}

func (r *TenantRepository) FindByID(_ context.Context, id string) (*spec.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tenant := r.tenants[id]; tenant != nil {
		cloned := *tenant
		return &cloned, nil
	}
	return nil, nil
}

func (r *TenantRepository) FindAll(_ context.Context) ([]*spec.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		cloned := *tenant
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.Tenant) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (r *TenantRepository) Save(_ context.Context, tenant *spec.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *tenant
	r.tenants[tenant.ID] = &cloned
	return nil
}
