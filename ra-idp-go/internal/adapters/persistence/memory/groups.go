package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// GroupRepository (ADR-038)
// =====================================================================

type GroupRepository struct {
	mu      sync.RWMutex
	groups  map[string]*spec.Group         // key: tenantKey(tenant_id, id)
	members map[string][]*spec.GroupMember // key: tenantKey(tenant_id, group_id)
}

func NewGroupRepository() *GroupRepository {
	return &GroupRepository{
		groups:  map[string]*spec.Group{},
		members: map[string][]*spec.GroupMember{},
	}
}

func cloneGroup(group *spec.Group) *spec.Group {
	cloned := *group
	cloned.Roles = slices.Clone(group.Roles)
	return &cloned
}

func (r *GroupRepository) ListByTenant(_ context.Context, tenantID string) ([]*spec.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Group, 0)
	for _, group := range r.groups {
		if group.TenantID == tenantID {
			out = append(out, cloneGroup(group))
		}
	}
	slices.SortFunc(out, func(a, b *spec.Group) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *GroupRepository) FindByID(_ context.Context, tenantID, id string) (*spec.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	group := r.groups[tenantKey(tenantID, id)]
	if group == nil {
		return nil, nil
	}
	return cloneGroup(group), nil
}

func (r *GroupRepository) Save(_ context.Context, group *spec.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[tenantKey(group.TenantID, group.ID)] = cloneGroup(group)
	return nil
}

func (r *GroupRepository) Delete(_ context.Context, tenantID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.groups, tenantKey(tenantID, id))
	delete(r.members, tenantKey(tenantID, id))
	return nil
}

func (r *GroupRepository) ListMembersByGroup(_ context.Context, tenantID, groupID string) ([]*spec.GroupMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stored := r.members[tenantKey(tenantID, groupID)]
	out := make([]*spec.GroupMember, 0, len(stored))
	for _, member := range stored {
		cloned := *member
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.GroupMember) int { return strings.Compare(a.UserSub, b.UserSub) })
	return out, nil
}

func (r *GroupRepository) ListGroupsByUser(_ context.Context, tenantID, userSub string) ([]*spec.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Group, 0)
	for key, members := range r.members {
		if !slices.ContainsFunc(members, func(m *spec.GroupMember) bool { return m.UserSub == userSub }) {
			continue
		}
		group := r.groups[key]
		if group != nil && group.TenantID == tenantID {
			out = append(out, cloneGroup(group))
		}
	}
	slices.SortFunc(out, func(a, b *spec.Group) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *GroupRepository) CountMembers(_ context.Context, tenantID, groupID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members[tenantKey(tenantID, groupID)]), nil
}

func (r *GroupRepository) AddMember(_ context.Context, member *spec.GroupMember) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.memberKey(member.GroupID)
	for _, existing := range r.members[key] {
		if existing.UserSub == member.UserSub {
			return false, nil
		}
	}
	cloned := *member
	r.members[key] = append(r.members[key], &cloned)
	return true, nil
}

func (r *GroupRepository) RemoveMember(_ context.Context, tenantID, groupID, userSub string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := tenantKey(tenantID, groupID)
	members := r.members[key]
	for i, existing := range members {
		if existing.UserSub == userSub {
			r.members[key] = slices.Delete(members, i, i+1)
			return true, nil
		}
	}
	return false, nil
}

// memberKey は group_id から所属する group のテナントを解決して member マップの
// キーを作る。GroupMember は tenant_id を持たないため group から引く。
func (r *GroupRepository) memberKey(groupID string) string {
	for key, group := range r.groups {
		if group.ID == groupID {
			return key
		}
	}
	return groupID
}
