package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// ConsentRepository (OAuth2)
// =====================================================================

type ConsentRepository struct {
	mu       sync.RWMutex
	consents map[string]*spec.Consent
}

func NewConsentRepository() *ConsentRepository {
	return &ConsentRepository{consents: map[string]*spec.Consent{}}
}

func (r *ConsentRepository) Find(_ context.Context, tenantID, sub, clientID string) (*spec.Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	consent := r.consents[consentKey(tenantID, sub, clientID)]
	if consent == nil {
		return nil, nil
	}
	cloned := *consent
	cloned.Scopes = slices.Clone(consent.Scopes)
	return &cloned, nil
}

func (r *ConsentRepository) FindAll(_ context.Context, tenantID string) ([]*spec.Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Consent, 0)
	for _, consent := range r.consents {
		if consent.TenantID != tenantID {
			continue
		}
		cloned := *consent
		cloned.Scopes = slices.Clone(consent.Scopes)
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.Consent) int {
		if a.Sub != b.Sub {
			return strings.Compare(a.Sub, b.Sub)
		}
		return strings.Compare(a.ClientID, b.ClientID)
	})
	return out, nil
}

func (r *ConsentRepository) Save(_ context.Context, c *spec.Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *c
	defaultTenant(&cloned.TenantID)
	cloned.Scopes = slices.Clone(c.Scopes)
	r.consents[consentKey(cloned.TenantID, cloned.Sub, cloned.ClientID)] = &cloned
	return nil
}

func (r *ConsentRepository) Revoke(_ context.Context, tenantID, sub, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.consents[consentKey(tenantID, sub, clientID)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	c.State = spec.ConsentRevoked
	c.RevokedAt = &now
	return nil
}

func (r *ConsentRepository) DeleteAllForSub(_ context.Context, sub string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, consent := range r.consents {
		if consent.Sub == sub {
			delete(r.consents, key)
		}
	}
	return nil
}

func consentKey(tenantID, sub, clientID string) string {
	return tenantKey(tenantID, sub+"|"+clientID)
}
