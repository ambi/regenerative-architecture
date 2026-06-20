package usecases

// 管理者向け Group ライフサイクル操作と user-group membership (ADR-038)。
// SCL Authentication component が所有する admin インターフェース群:
// ListGroups / GetGroup / CreateGroup / UpdateGroup / DeleteGroup /
// AddGroupMember / RemoveGroupMember / ListUserGroups。
//
// すべての操作は tenancy.TenantID(ctx) のテナント境界に閉じ、cross-tenant な
// 参照・所属は reject する。effective_roles = union(user.roles, group.roles)。

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var (
	ErrGroupNotFound     = errors.New("group not found")
	ErrGroupNameConflict = errors.New("group name already exists")
	ErrGroupNameEmpty    = errors.New("group name is required")
)

type AdminGroupDeps struct {
	GroupRepo authports.GroupRepository
	UserRepo  oauthports.UserRepository
	Emit      func(spec.DomainEvent)
}

// GroupView は一覧・詳細でグループとメンバー数をまとめて返す。
type GroupView struct {
	Group       *spec.Group
	MemberCount int
}

func ListGroups(ctx context.Context, deps AdminGroupDeps) ([]GroupView, error) {
	tenantID := tenancy.TenantID(ctx)
	groups, err := deps.GroupRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]GroupView, 0, len(groups))
	for _, group := range groups {
		count, err := deps.GroupRepo.CountMembers(ctx, tenantID, group.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, GroupView{Group: group, MemberCount: count})
	}
	return views, nil
}

// GetGroup はグループ本体と所属メンバー一覧を返す。別テナントのグループは
// 未存在として扱う。
func GetGroup(ctx context.Context, deps AdminGroupDeps, id string) (*spec.Group, []*spec.GroupMember, error) {
	tenantID := tenancy.TenantID(ctx)
	group, err := deps.GroupRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	if group == nil {
		return nil, nil, ErrGroupNotFound
	}
	members, err := deps.GroupRepo.ListMembersByGroup(ctx, tenantID, id)
	if err != nil {
		return nil, nil, err
	}
	return group, members, nil
}

type CreateGroupInput struct {
	ActorSub    string
	Name        string
	Description *string
	Roles       []string
	Now         time.Time
}

func CreateGroup(ctx context.Context, deps AdminGroupDeps, in CreateGroupInput) (*spec.Group, error) {
	tenantID := tenancy.TenantID(ctx)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrGroupNameEmpty
	}
	if err := ensureGroupNameAvailable(ctx, deps, tenantID, name, ""); err != nil {
		return nil, err
	}
	roles, err := normalizeRoles(in.Roles)
	if err != nil {
		return nil, err
	}
	id, err := spec.NewGroupID()
	if err != nil {
		return nil, err
	}
	now := normalizedNow(in.Now)
	group := &spec.Group{
		ID: id, TenantID: tenantID, Name: name, Description: normalizeDescription(in.Description),
		Roles: roles, CreatedAt: now,
	}
	if err := group.Validate(); err != nil {
		return nil, err
	}
	if err := deps.GroupRepo.Save(ctx, group); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.GroupCreated{At: now, TenantID: group.TenantID, ActorSub: in.ActorSub, GroupID: group.ID})
	return group, nil
}

type UpdateGroupInput struct {
	ActorSub    string
	ID          string
	Name        *string
	Description *string
	Roles       *[]string
	Now         time.Time
}

func UpdateGroup(ctx context.Context, deps AdminGroupDeps, in UpdateGroupInput) (*spec.Group, error) {
	tenantID := tenancy.TenantID(ctx)
	group, err := deps.GroupRepo.FindByID(ctx, tenantID, in.ID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	updated := *group
	changed := []string{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, ErrGroupNameEmpty
		}
		if name != group.Name {
			if err := ensureGroupNameAvailable(ctx, deps, tenantID, name, group.ID); err != nil {
				return nil, err
			}
			updated.Name = name
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		desc := normalizeDescription(in.Description)
		if !equalOptionalString(group.Description, desc) {
			updated.Description = desc
			changed = append(changed, "description")
		}
	}
	if in.Roles != nil {
		roles, err := normalizeRoles(*in.Roles)
		if err != nil {
			return nil, err
		}
		if !slices.Equal(roles, group.Roles) {
			updated.Roles = roles
			changed = append(changed, "roles")
		}
	}
	if len(changed) == 0 {
		return &updated, nil
	}
	now := normalizedNow(in.Now)
	updated.UpdatedAt = &now
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.GroupRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	adminEmit(deps.Emit, &spec.GroupUpdated{
		At: now, TenantID: group.TenantID, ActorSub: in.ActorSub, GroupID: group.ID, ChangedFields: changed,
	})
	return &updated, nil
}

// DeleteGroup はグループを物理削除し、所属 membership を cascade で解除する。
// 解除メンバーごとに GroupMemberRemoved を emit し、最後に GroupDeleted を emit する。
func DeleteGroup(ctx context.Context, deps AdminGroupDeps, actorSub, id string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	group, err := deps.GroupRepo.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	members, err := deps.GroupRepo.ListMembersByGroup(ctx, tenantID, id)
	if err != nil {
		return err
	}
	now = normalizedNow(now)
	for _, member := range members {
		removed, err := deps.GroupRepo.RemoveMember(ctx, tenantID, id, member.UserSub)
		if err != nil {
			return err
		}
		if removed {
			adminEmit(deps.Emit, &spec.GroupMemberRemoved{
				At: now, TenantID: tenantID, ActorSub: actorSub, GroupID: id, UserSub: member.UserSub,
			})
		}
	}
	if err := deps.GroupRepo.Delete(ctx, tenantID, id); err != nil {
		return err
	}
	adminEmit(deps.Emit, &spec.GroupDeleted{At: now, TenantID: tenantID, ActorSub: actorSub, GroupID: id})
	return nil
}

// AddMember は同一テナントの User をグループに所属させる。既所属なら no-op で
// イベントも emit しない (冪等)。
func AddMember(ctx context.Context, deps AdminGroupDeps, actorSub, groupID, userSub string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	group, err := deps.GroupRepo.FindByID(ctx, tenantID, groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	user, err := deps.UserRepo.FindBySub(ctx, userSub)
	if err != nil {
		return err
	}
	if user == nil || user.TenantID != tenantID {
		return ErrUserNotFound
	}
	now = normalizedNow(now)
	added, err := deps.GroupRepo.AddMember(ctx, &spec.GroupMember{
		GroupID: groupID, UserSub: userSub, AddedAt: now,
	})
	if err != nil {
		return err
	}
	if added {
		adminEmit(deps.Emit, &spec.GroupMemberAdded{
			At: now, TenantID: tenantID, ActorSub: actorSub, GroupID: groupID, UserSub: userSub,
		})
	}
	return nil
}

// RemoveMember はグループから User を外す。非所属なら no-op で event も emit しない。
func RemoveMember(ctx context.Context, deps AdminGroupDeps, actorSub, groupID, userSub string, now time.Time) error {
	tenantID := tenancy.TenantID(ctx)
	group, err := deps.GroupRepo.FindByID(ctx, tenantID, groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	now = normalizedNow(now)
	removed, err := deps.GroupRepo.RemoveMember(ctx, tenantID, groupID, userSub)
	if err != nil {
		return err
	}
	if removed {
		adminEmit(deps.Emit, &spec.GroupMemberRemoved{
			At: now, TenantID: tenantID, ActorSub: actorSub, GroupID: groupID, UserSub: userSub,
		})
	}
	return nil
}

// UserGroupView は ListUserGroups の結果。明示ロール・グループ由来ロール・union を
// 分けて返し、管理 UI が effective roles を理解しやすくする。
type UserGroupView struct {
	Groups         []*spec.Group
	DirectRoles    []string
	GroupRoles     []string
	EffectiveRoles []string
}

func UserGroups(ctx context.Context, deps AdminGroupDeps, sub string) (*UserGroupView, error) {
	tenantID := tenancy.TenantID(ctx)
	user, err := deps.UserRepo.FindBySub(ctx, sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.TenantID != tenantID {
		return nil, ErrUserNotFound
	}
	groups, err := deps.GroupRepo.ListGroupsByUser(ctx, tenantID, sub)
	if err != nil {
		return nil, err
	}
	directRoles := spec.EffectiveRoles(user.Roles, nil)
	groupRoles := spec.EffectiveRoles(nil, groups)
	return &UserGroupView{
		Groups:         groups,
		DirectRoles:    directRoles,
		GroupRoles:     groupRoles,
		EffectiveRoles: spec.EffectiveRoles(user.Roles, groups),
	}, nil
}

func ensureGroupNameAvailable(ctx context.Context, deps AdminGroupDeps, tenantID, name, excludeID string) error {
	groups, err := deps.GroupRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.ID != excludeID && strings.EqualFold(group.Name, name) {
			return ErrGroupNameConflict
		}
	}
	return nil
}

func normalizeDescription(description *string) *string {
	if description == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*description)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
