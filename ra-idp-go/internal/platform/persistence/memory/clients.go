package memory

import (
	"context"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// ClientRepository (OAuth2)
// =====================================================================

type ClientRepository struct {
	mu      sync.RWMutex
	clients map[string]*spec.Client
}

func NewClientRepository() *ClientRepository {
	return &ClientRepository{clients: map[string]*spec.Client{}}
}

func (r *ClientRepository) Seed(c *spec.Client) {
	_ = r.Save(context.Background(), c)
}

func (r *ClientRepository) FindByID(_ context.Context, tenantID, clientID string) (*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[tenantKey(tenantID, clientID)], nil
}

func (r *ClientRepository) Save(_ context.Context, c *spec.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defaultTenant(&c.TenantID)
	r.clients[tenantKey(c.TenantID, c.ClientID)] = c
	return nil
}

func (r *ClientRepository) Delete(_ context.Context, tenantID, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, tenantKey(tenantID, clientID))
	return nil
}

func (r *ClientRepository) FindAll(_ context.Context, tenantID string) ([]*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Client, 0, len(r.clients))
	for _, c := range r.clients {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}
