package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// AgentRepository (ADR-048)
// =====================================================================

type AgentRepository struct {
	mu       sync.RWMutex
	agents   map[string]*spec.Agent                    // key: tenantKey(tenant_id, id)
	bindings map[string][]*spec.AgentCredentialBinding // key: tenantKey(tenant_id, agent_id)
}

func NewAgentRepository() *AgentRepository {
	return &AgentRepository{
		agents:   map[string]*spec.Agent{},
		bindings: map[string][]*spec.AgentCredentialBinding{},
	}
}

func cloneAgent(agent *spec.Agent) *spec.Agent {
	cloned := *agent
	cloned.Roles = slices.Clone(agent.Roles)
	return &cloned
}

func (r *AgentRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Agent, 0)
	for _, agent := range r.agents {
		if agent.TenantID == tenantID {
			out = append(out, cloneAgent(agent))
		}
	}
	slices.SortFunc(out, func(a, b *spec.Agent) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *AgentRepository) FindByID(_ context.Context, tenantID, id string) (*spec.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent := r.agents[tenantKey(tenantID, id)]
	if agent == nil {
		return nil, nil
	}
	return cloneAgent(agent), nil
}

func (r *AgentRepository) Save(_ context.Context, agent *spec.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[tenantKey(agent.TenantID, agent.ID)] = cloneAgent(agent)
	return nil
}

func (r *AgentRepository) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, tenantKey(tenantID, id))
	delete(r.bindings, tenantKey(tenantID, id))
	return nil
}

func (r *AgentRepository) ListBindings(_ context.Context, tenantID, agentID string) ([]*spec.AgentCredentialBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stored := r.bindings[tenantKey(tenantID, agentID)]
	out := make([]*spec.AgentCredentialBinding, 0, len(stored))
	for _, binding := range stored {
		cloned := *binding
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.AgentCredentialBinding) int { return strings.Compare(a.ClientID, b.ClientID) })
	return out, nil
}

func (r *AgentRepository) AddBinding(_ context.Context, binding *spec.AgentCredentialBinding) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := tenantKey(binding.TenantID, binding.AgentID)
	for _, existing := range r.bindings[key] {
		if existing.ClientID == binding.ClientID {
			return false, nil
		}
	}
	cloned := *binding
	r.bindings[key] = append(r.bindings[key], &cloned)
	return true, nil
}

func (r *AgentRepository) RemoveBinding(_ context.Context, tenantID, agentID, clientID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := tenantKey(tenantID, agentID)
	bindings := r.bindings[key]
	for i, existing := range bindings {
		if existing.ClientID == clientID {
			r.bindings[key] = slices.Delete(bindings, i, i+1)
			return true, nil
		}
	}
	return false, nil
}

func (r *AgentRepository) FindByClientID(_ context.Context, tenantID, clientID string) (*spec.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, bindings := range r.bindings {
		if !slices.ContainsFunc(bindings, func(b *spec.AgentCredentialBinding) bool { return b.ClientID == clientID }) {
			continue
		}
		agent := r.agents[key]
		if agent != nil && agent.TenantID == tenantID {
			return cloneAgent(agent), nil
		}
	}
	return nil, nil
}
