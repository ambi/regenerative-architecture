package memory

import (
	"context"
	"slices"
	"sync"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// TenantUserAttributeSchemaRepository (ADR-040 / wi-19)
// =====================================================================

type TenantUserAttributeSchemaRepository struct {
	mu       sync.RWMutex
	byTenant map[string]*spec.TenantUserAttributeSchema
}

func NewTenantUserAttributeSchemaRepository() *TenantUserAttributeSchemaRepository {
	return &TenantUserAttributeSchemaRepository{byTenant: map[string]*spec.TenantUserAttributeSchema{}}
}

func (r *TenantUserAttributeSchemaRepository) FindByTenant(_ context.Context, tenantID string) (*spec.TenantUserAttributeSchema, error) {
	defaultTenant(&tenantID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if schema := r.byTenant[tenantID]; schema != nil {
		return cloneUserAttributeSchema(schema), nil
	}
	return nil, nil
}

func (r *TenantUserAttributeSchemaRepository) Save(_ context.Context, schema *spec.TenantUserAttributeSchema) error {
	cloned := cloneUserAttributeSchema(schema)
	defaultTenant(&cloned.TenantID)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byTenant[cloned.TenantID] = cloned
	return nil
}

func (r *TenantUserAttributeSchemaRepository) Delete(_ context.Context, tenantID string) error {
	defaultTenant(&tenantID)
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byTenant, tenantID)
	return nil
}

// cloneUserAttributeSchema は呼び出し側との aliasing を断つための深いコピー。
func cloneUserAttributeSchema(s *spec.TenantUserAttributeSchema) *spec.TenantUserAttributeSchema {
	cloned := *s
	cloned.Attributes = slices.Clone(s.Attributes)
	return &cloned
}
