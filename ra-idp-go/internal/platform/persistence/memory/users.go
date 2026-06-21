package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// UserRepository (Authentication)
// =====================================================================

type UserRepository struct {
	mu     sync.RWMutex
	bySub  map[string]*spec.User
	byUser map[string]*spec.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{bySub: map[string]*spec.User{}, byUser: map[string]*spec.User{}}
}

func (r *UserRepository) Seed(u *spec.User) {
	_ = r.Save(context.Background(), u)
}

func (r *UserRepository) Save(_ context.Context, u *spec.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing := r.bySub[u.Sub]; existing != nil &&
		existing.PreferredUsername != u.PreferredUsername {
		delete(r.byUser, tenantKey(existing.TenantID, existing.PreferredUsername))
	}
	defaultTenant(&u.TenantID)
	r.bySub[u.Sub] = u
	r.byUser[tenantKey(u.TenantID, u.PreferredUsername)] = u
	return nil
}

func (r *UserRepository) FindBySub(_ context.Context, sub string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user := r.bySub[sub]
	if user == nil || user.IsDeleted() {
		return nil, nil
	}
	return user, nil
}

func (r *UserRepository) FindBySubIncludingDeleted(_ context.Context, sub string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bySub[sub], nil
}

func (r *UserRepository) FindByUsername(_ context.Context, tenantID, username string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user := r.byUser[tenantKey(tenantID, username)]
	if user == nil || user.IsDeleted() {
		return nil, nil
	}
	return user, nil
}

func (r *UserRepository) FindByEmail(_ context.Context, tenantID, email string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.bySub {
		if user.IsDeleted() {
			continue
		}
		if user.TenantID == tenantID && user.Email != nil && strings.EqualFold(*user.Email, email) {
			return user, nil
		}
	}
	return nil, nil
}

func (r *UserRepository) FindAll(_ context.Context, tenantID string) ([]*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.User, 0, len(r.bySub))
	for _, user := range r.bySub {
		if user.TenantID == tenantID && !user.IsDeleted() {
			out = append(out, user)
		}
	}
	slices.SortFunc(out, func(a, b *spec.User) int {
		return strings.Compare(a.PreferredUsername, b.PreferredUsername)
	})
	return out, nil
}
