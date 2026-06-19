package http

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type groupCreateRequest struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Roles       []string `json:"roles"`
}

type groupUpdateRequest struct {
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Roles       *[]string `json:"roles"`
}

type groupSummaryResponse struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Roles       []string   `json:"roles"`
	MemberCount int        `json:"member_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type groupMemberResponse struct {
	UserSub           string    `json:"user_sub"`
	PreferredUsername string    `json:"preferred_username"`
	AddedAt           time.Time `json:"added_at"`
}

type userGroupsResponse struct {
	Groups         []groupSummaryResponse `json:"groups"`
	DirectRoles    []string               `json:"direct_roles"`
	GroupRoles     []string               `json:"group_roles"`
	EffectiveRoles []string               `json:"effective_roles"`
}

func (d Deps) handleListGroups(c *echo.Context) error {
	if _, err := d.requireAdmin(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	views, err := authusecases.ListGroups(c.Request().Context(), d.adminGroupDeps())
	if err != nil {
		return err
	}
	groups := make([]groupSummaryResponse, len(views))
	for i, view := range views {
		groups[i] = toGroupSummaryResponse(view.Group, view.MemberCount)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{"groups": groups})
}

func (d Deps) handleGetGroup(c *echo.Context) error {
	if _, err := d.requireAdmin(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	group, members, err := authusecases.GetGroup(c.Request().Context(), d.adminGroupDeps(), c.Param("group_id"))
	if err != nil {
		return d.writeAdminGroupError(c, err)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{
		"group":   toGroupSummaryResponse(group, len(members)),
		"members": d.toGroupMemberResponses(c.Request().Context(), members),
	})
}

func (d Deps) handleCreateGroup(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	var input groupCreateRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	group, err := authusecases.CreateGroup(c.Request().Context(), d.adminGroupDeps(), authusecases.CreateGroupInput{
		ActorSub: actor.Sub, Name: input.Name, Description: input.Description, Roles: input.Roles, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminGroupError(c, err)
	}
	return noStoreJSON(c, http.StatusCreated, toGroupSummaryResponse(group, 0))
}

func (d Deps) handleUpdateGroup(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	var input groupUpdateRequest
	if err := decodeJSON(c.Request(), &input); err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	group, err := authusecases.UpdateGroup(c.Request().Context(), d.adminGroupDeps(), authusecases.UpdateGroupInput{
		ActorSub: actor.Sub, ID: c.Param("group_id"),
		Name: input.Name, Description: input.Description, Roles: input.Roles, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminGroupError(c, err)
	}
	count, err := d.adminGroupDeps().GroupRepo.CountMembers(c.Request().Context(), group.TenantID, group.ID)
	if err != nil {
		return err
	}
	return noStoreJSON(c, http.StatusOK, toGroupSummaryResponse(group, count))
}

func (d Deps) handleDeleteGroup(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if err := authusecases.DeleteGroup(c.Request().Context(), d.adminGroupDeps(), actor.Sub, c.Param("group_id"), time.Now().UTC()); err != nil {
		return d.writeAdminGroupError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleAddGroupMember(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if err := authusecases.AddMember(c.Request().Context(), d.adminGroupDeps(), actor.Sub, c.Param("group_id"), c.Param("user_sub"), time.Now().UTC()); err != nil {
		return d.writeAdminGroupError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleRemoveGroupMember(c *echo.Context) error {
	if err := d.verifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.requireAdmin(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if err := authusecases.RemoveMember(c.Request().Context(), d.adminGroupDeps(), actor.Sub, c.Param("group_id"), c.Param("user_sub"), time.Now().UTC()); err != nil {
		return d.writeAdminGroupError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleListUserGroups(c *echo.Context) error {
	if _, err := d.requireAdmin(c); err != nil {
		return d.writeAdminAccessError(c, err)
	}
	view, err := authusecases.UserGroups(c.Request().Context(), d.adminGroupDeps(), c.Param("sub"))
	if err != nil {
		return d.writeAdminGroupError(c, err)
	}
	groups := make([]groupSummaryResponse, len(view.Groups))
	for i, group := range view.Groups {
		count, err := d.GroupRepo.CountMembers(c.Request().Context(), group.TenantID, group.ID)
		if err != nil {
			return err
		}
		groups[i] = toGroupSummaryResponse(group, count)
	}
	return noStoreJSON(c, http.StatusOK, userGroupsResponse{
		Groups:         groups,
		DirectRoles:    view.DirectRoles,
		GroupRoles:     view.GroupRoles,
		EffectiveRoles: view.EffectiveRoles,
	})
}

func (d Deps) adminGroupDeps() authusecases.AdminGroupDeps {
	return authusecases.AdminGroupDeps{GroupRepo: d.GroupRepo, UserRepo: d.UserRepo, Emit: d.Emit}
}

func (d Deps) toGroupMemberResponses(ctx context.Context, members []*spec.GroupMember) []groupMemberResponse {
	out := make([]groupMemberResponse, len(members))
	for i, member := range members {
		username := member.UserSub
		if user, err := d.UserRepo.FindBySub(ctx, member.UserSub); err == nil && user != nil {
			username = user.PreferredUsername
		}
		out[i] = groupMemberResponse{UserSub: member.UserSub, PreferredUsername: username, AddedAt: member.AddedAt}
	}
	return out
}

func toGroupSummaryResponse(group *spec.Group, memberCount int) groupSummaryResponse {
	return groupSummaryResponse{
		ID: group.ID, TenantID: group.TenantID, Name: group.Name, Description: group.Description,
		Roles: slices.Clone(group.Roles), MemberCount: memberCount,
		CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
	}
}

func (d Deps) writeAdminGroupError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, authusecases.ErrGroupNotFound):
		return writeBrowserError(c, http.StatusNotFound, "group_not_found", "グループが存在しません")
	case errors.Is(err, authusecases.ErrUserNotFound):
		return writeBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	case errors.Is(err, authusecases.ErrGroupNameConflict):
		return writeBrowserError(c, http.StatusConflict, "group_name_conflict", "グループ名は既に使用されています")
	case errors.Is(err, authusecases.ErrGroupNameEmpty):
		return writeBrowserError(c, http.StatusBadRequest, "group_name_required", "グループ名は必須です")
	case errors.Is(err, authusecases.ErrInvalidRole):
		return writeBrowserError(c, http.StatusBadRequest, "invalid_role", "roleが不正です")
	default:
		return err
	}
}

// effectiveRoles は actor の有効ロール (user.roles ∪ 所属 group.roles) を返す (ADR-038)。
// GroupRepo 未配線時は user.roles をそのまま返し、後方互換を保つ。
func (d Deps) effectiveRoles(ctx context.Context, user *spec.User) []string {
	if d.GroupRepo == nil {
		return user.Roles
	}
	groups, err := d.GroupRepo.ListGroupsByUser(ctx, user.TenantID, user.Sub)
	if err != nil {
		return user.Roles
	}
	return spec.EffectiveRoles(user.Roles, groups)
}

// withEffectiveRoles は user のコピーに有効ロールを載せて返す (ADR-038)。
// admin actor を解決する各経路 (settings / role policy / key / audit) が
// グループ由来ロールを一貫して評価できるようにする。
func (d Deps) withEffectiveRoles(ctx context.Context, user *spec.User) *spec.User {
	clone := *user
	clone.Roles = d.effectiveRoles(ctx, user)
	return &clone
}
