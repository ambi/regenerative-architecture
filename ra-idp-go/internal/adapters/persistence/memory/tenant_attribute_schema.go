package memory

import (
	"context"
	"slices"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// TenantAttributeSchemaRepository (ADR-040 / wi-19)
// =====================================================================

type TenantAttributeSchemaRepository struct {
	mu       sync.RWMutex
	byTenant map[string]*spec.TenantAttributeSchema
}

func NewTenantAttributeSchemaRepository() *TenantAttributeSchemaRepository {
	return &TenantAttributeSchemaRepository{byTenant: map[string]*spec.TenantAttributeSchema{}}
}

func (r *TenantAttributeSchemaRepository) FindByTenant(_ context.Context, tenantID string) (*spec.TenantAttributeSchema, error) {
	defaultTenant(&tenantID)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if schema := r.byTenant[tenantID]; schema != nil {
		return cloneAttributeSchema(schema), nil
	}
	return nil, nil
}

func (r *TenantAttributeSchemaRepository) Save(_ context.Context, schema *spec.TenantAttributeSchema) error {
	cloned := cloneAttributeSchema(schema)
	defaultTenant(&cloned.TenantID)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byTenant[cloned.TenantID] = cloned
	return nil
}

func (r *TenantAttributeSchemaRepository) Delete(_ context.Context, tenantID string) error {
	defaultTenant(&tenantID)
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byTenant, tenantID)
	return nil
}

// cloneAttributeSchema は呼び出し側との aliasing を断つための深いコピー。
func cloneAttributeSchema(s *spec.TenantAttributeSchema) *spec.TenantAttributeSchema {
	cloned := *s
	cloned.Attributes = slices.Clone(s.Attributes)
	return &cloned
}
