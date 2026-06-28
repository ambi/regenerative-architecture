package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// SamlServiceProviderRepository (SAML 2.0 Web Browser SSO, wi-29)
// =====================================================================

type SamlServiceProviderRepository struct {
	mu  sync.RWMutex
	sps map[string]*spec.SamlServiceProvider // key: tenantKey(tenant_id, entity_id)
}

func NewSamlServiceProviderRepository() *SamlServiceProviderRepository {
	return &SamlServiceProviderRepository{sps: map[string]*spec.SamlServiceProvider{}}
}

// Seed は起動時のサンプル SP 投入に使う (テスト・デモ用)。
func (r *SamlServiceProviderRepository) Seed(sp *spec.SamlServiceProvider) {
	_ = r.Save(context.Background(), sp)
}

func cloneServiceProvider(sp *spec.SamlServiceProvider) *spec.SamlServiceProvider {
	cloned := *sp
	cloned.ACSURLs = slices.Clone(sp.ACSURLs)
	cloned.ClaimPolicy.Rules = slices.Clone(sp.ClaimPolicy.Rules)
	return &cloned
}

func (r *SamlServiceProviderRepository) FindByEntityID(_ context.Context, tenantID, entityID string) (*spec.SamlServiceProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sp := r.sps[tenantKey(tenantID, entityID)]
	if sp == nil {
		return nil, nil
	}
	return cloneServiceProvider(sp), nil
}

func (r *SamlServiceProviderRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.SamlServiceProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.SamlServiceProvider, 0)
	for _, sp := range r.sps {
		if sp.TenantID == tenantID {
			out = append(out, cloneServiceProvider(sp))
		}
	}
	slices.SortFunc(out, func(a, b *spec.SamlServiceProvider) int { return strings.Compare(a.EntityID, b.EntityID) })
	return out, nil
}

func (r *SamlServiceProviderRepository) Save(_ context.Context, sp *spec.SamlServiceProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defaultTenant(&sp.TenantID)
	r.sps[tenantKey(sp.TenantID, sp.EntityID)] = cloneServiceProvider(sp)
	return nil
}

func (r *SamlServiceProviderRepository) Delete(_ context.Context, tenantID, entityID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sps, tenantKey(tenantID, entityID))
	return nil
}
