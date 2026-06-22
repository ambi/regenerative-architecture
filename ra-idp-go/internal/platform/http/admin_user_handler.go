package http

import (
	"errors"
	"net/http"
	"slices"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type adminUserCreateRequest struct {
	PreferredUsername string   `json:"preferred_username"`
	Password          string   `json:"password"`
	Name              *string  `json:"name"`
	Email             *string  `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Roles             []string `json:"roles"`
}

type adminUserUpdateRequest struct {
	PreferredUsername *string                         `json:"preferred_username"`
	Name              *string                         `json:"name"`
	GivenName         *string                         `json:"given_name"`
	FamilyName        *string                         `json:"family_name"`
	Email             *string                         `json:"email"`
	EmailVerified     *bool                           `json:"email_verified"`
	Roles             *[]string                       `json:"roles"`
	Attributes        *map[string]spec.AttributeValue `json:"attributes"`
}

type adminUserDeleteRequest struct {
	Reason string `json:"reason"`
}

type adminUserResponse struct {
	Sub               string                         `json:"sub"`
	PreferredUsername string                         `json:"preferred_username"`
	Name              *string                        `json:"name,omitempty"`
	GivenName         *string                        `json:"given_name,omitempty"`
	FamilyName        *string                        `json:"family_name,omitempty"`
	Email             *string                        `json:"email,omitempty"`
	EmailVerified     bool                           `json:"email_verified"`
	MfaEnrolled       bool                           `json:"mfa_enrolled"`
	Roles             []string                       `json:"roles"`
	Status            spec.UserStatus                `json:"status"`
	Attributes        map[string]spec.AttributeValue `json:"attributes,omitempty"`
	RequiredActions   []spec.RequiredAction          `json:"required_actions,omitempty"`
	LastLoginAt       *time.Time                     `json:"last_login_at,omitempty"`
	PasswordChangedAt *time.Time                     `json:"password_changed_at,omitempty"`
	// DisabledAt は status から導出した後方互換フィールド (現行 UI 用)。
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type adminRequiredActionRequest struct {
	Action string `json:"action"`
}

func (d Deps) handleListAdminUsers(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	users, err := d.UserRepo.FindAll(c.Request().Context(), core.RequestTenantID(c))
	if err != nil {
		return err
	}
	response := make([]adminUserResponse, len(users))
	for i, user := range users {
		response[i] = toAdminUserResponse(user)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"users": response})
}

func (d Deps) handleGetAdminUser(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), c.Param("sub"))
	if err != nil {
		return err
	}
	if user == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	}
	if user.TenantID != core.RequestTenantID(c) {
		return core.WriteBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminUserResponse(user))
}

func (d Deps) handleCreateAdminUser(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input adminUserCreateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	user, err := authusecases.CreateUser(
		c.Request().Context(),
		d.adminUserDeps(),
		authusecases.CreateUserInput{
			ActorSub: actor.Sub, PreferredUsername: input.PreferredUsername,
			Password: input.Password, Name: input.Name, Email: input.Email,
			EmailVerified: input.EmailVerified, Roles: input.Roles, Now: time.Now().UTC(),
		},
	)
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusCreated, toAdminUserResponse(user))
}

func (d Deps) handleUpdateAdminUser(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input adminUserUpdateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	user, err := authusecases.UpdateUser(
		c.Request().Context(),
		d.adminUserDeps(),
		authusecases.UpdateUserInput{
			ActorSub: actor.Sub, Sub: c.Param("sub"),
			PreferredUsername: input.PreferredUsername, Name: input.Name,
			GivenName: input.GivenName, FamilyName: input.FamilyName, Email: input.Email,
			EmailVerified: input.EmailVerified, Roles: input.Roles,
			Attributes: input.Attributes, Now: time.Now().UTC(),
		},
	)
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminUserResponse(user))
}

func (d Deps) handleDisableAdminUser(c *echo.Context) error {
	return d.handleSetAdminUserDisabled(c, true)
}

func (d Deps) handleEnableAdminUser(c *echo.Context) error {
	return d.handleSetAdminUserDisabled(c, false)
}

func (d Deps) handleDeleteAdminUser(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input adminUserDeleteRequest
	if c.Request().ContentLength > 0 {
		if err := core.DecodeJSON(c.Request(), &input); err != nil {
			return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
		}
	}
	err = authusecases.DeleteUser(c.Request().Context(), d.adminUserDeps(), authusecases.DeleteUserInput{
		ActorSub: actor.Sub, Sub: c.Param("sub"), Reason: input.Reason, Now: time.Now().UTC(),
	})
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleSetAdminUserDisabled(c *echo.Context, disabled bool) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	_, err = authusecases.SetUserDisabled(
		c.Request().Context(), d.adminUserDeps(), actor.Sub, c.Param("sub"), disabled, time.Now().UTC(),
	)
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) adminUserDeps() authusecases.AdminUserDeps {
	deps := authusecases.AdminUserDeps{
		UserRepo: d.UserRepo, AttrSchemaRepo: d.AttrSchemaRepo,
		ConsentRepo: d.ConsentRepo, RefreshStore: d.RefreshStore,
		DeviceCodeStore: d.DeviceCodeStore, MfaFactorRepo: d.MfaFactorRepo,
		PasswordHasher: d.PasswordHasher, PasswordHistoryRepo: d.PasswordHistoryRepo,
		Emit: d.Emit,
	}
	if d.SessionManager != nil {
		deps.SessionStore = d.SessionManager.Store
	}
	return deps
}

func (d Deps) writeAdminUserError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, authusecases.ErrUserNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	case errors.Is(err, authusecases.ErrUsernameConflict):
		return core.WriteBrowserError(c, http.StatusConflict, "username_conflict", "ユーザー名は既に使用されています")
	case errors.Is(err, authusecases.ErrInvalidRole):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_role", "roleが不正です")
	case errors.Is(err, authusecases.ErrSelfDeleteForbidden):
		return core.WriteBrowserError(c, http.StatusBadRequest, "self_delete_forbidden", "管理者は自身を削除できません")
	case errors.Is(err, authusecases.ErrInvalidAttribute):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_attribute", "属性がスキーマに適合していません")
	case errors.Is(err, authusecases.ErrInvalidRequiredAction):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_required_action", "required action が不正です")
	default:
		var policyErr *authusecases.PasswordPolicyError
		if errors.As(err, &policyErr) {
			violations := make([]string, len(policyErr.Violations))
			for i, violation := range policyErr.Violations {
				violations[i] = string(violation)
			}
			return core.NoStoreJSON(c, http.StatusBadRequest, map[string]any{
				"error": "password_policy", "message": "パスワードがセキュリティ要件を満たしていません。",
				"violations": violations,
			})
		}
		return err
	}
}

func toAdminUserResponse(user *spec.User) adminUserResponse {
	var disabledAt *time.Time
	if user.Lifecycle.Status == spec.UserStatusDisabled {
		disabledAt = user.Lifecycle.StatusChangedAt
	}
	return adminUserResponse{
		Sub: user.Sub, PreferredUsername: user.PreferredUsername, Name: user.Name,
		GivenName: user.GivenName, FamilyName: user.FamilyName,
		Email: user.Email, EmailVerified: user.EmailVerified, MfaEnrolled: user.MfaEnrolled,
		Roles: slices.Clone(user.Roles), Status: user.Lifecycle.EffectiveStatus(),
		Attributes:        user.Attributes,
		RequiredActions:   slices.Clone(user.Lifecycle.RequiredActions),
		LastLoginAt:       user.Lifecycle.LastLoginAt,
		PasswordChangedAt: user.Lifecycle.PasswordChangedAt,
		DisabledAt:        disabledAt,
		CreatedAt:         user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func (d Deps) handleSetUserRequiredAction(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var input adminRequiredActionRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	user, err := authusecases.SetUserRequiredAction(
		c.Request().Context(), d.adminUserDeps(), actor.Sub, c.Param("sub"),
		spec.RequiredAction(input.Action), time.Now().UTC(),
	)
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminUserResponse(user))
}

func (d Deps) handleClearUserRequiredAction(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	user, err := authusecases.ClearUserRequiredAction(
		c.Request().Context(), d.adminUserDeps(), actor.Sub, c.Param("sub"),
		spec.RequiredAction(c.Param("action")), time.Now().UTC(),
	)
	if err != nil {
		return d.writeAdminUserError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminUserResponse(user))
}
