package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// WsFedRelyingPartyRepository (WS-Federation passive, wi-61)
// =====================================================================

type WsFedRelyingPartyRepository struct {
	mu    sync.RWMutex
	parts map[string]*spec.WsFedRelyingParty // key: tenantKey(tenant_id, wtrealm)
}

func NewWsFedRelyingPartyRepository() *WsFedRelyingPartyRepository {
	return &WsFedRelyingPartyRepository{parts: map[string]*spec.WsFedRelyingParty{}}
}

// Seed は起動時のサンプル RP 投入に使う (テスト・デモ用)。
func (r *WsFedRelyingPartyRepository) Seed(rp *spec.WsFedRelyingParty) {
	_ = r.Save(context.Background(), rp)
}

func cloneRelyingParty(rp *spec.WsFedRelyingParty) *spec.WsFedRelyingParty {
	cloned := *rp
	cloned.ReplyURLs = slices.Clone(rp.ReplyURLs)
	cloned.ClaimPolicy.Rules = slices.Clone(rp.ClaimPolicy.Rules)
	return &cloned
}

func (r *WsFedRelyingPartyRepository) FindByWtrealm(_ context.Context, tenantID, wtrealm string) (*spec.WsFedRelyingParty, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rp := r.parts[tenantKey(tenantID, wtrealm)]
	if rp == nil {
		return nil, nil
	}
	return cloneRelyingParty(rp), nil
}

func (r *WsFedRelyingPartyRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.WsFedRelyingParty, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.WsFedRelyingParty, 0)
	for _, rp := range r.parts {
		if rp.TenantID == tenantID {
			out = append(out, cloneRelyingParty(rp))
		}
	}
	slices.SortFunc(out, func(a, b *spec.WsFedRelyingParty) int { return strings.Compare(a.Wtrealm, b.Wtrealm) })
	return out, nil
}

func (r *WsFedRelyingPartyRepository) Save(_ context.Context, rp *spec.WsFedRelyingParty) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defaultTenant(&rp.TenantID)
	r.parts[tenantKey(rp.TenantID, rp.Wtrealm)] = cloneRelyingParty(rp)
	return nil
}

func (r *WsFedRelyingPartyRepository) Delete(_ context.Context, tenantID, wtrealm string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.parts, tenantKey(tenantID, wtrealm))
	return nil
}
