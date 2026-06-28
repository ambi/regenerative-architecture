// /api/account/profile — エンドユーザ自身のプロフィール参照・編集 (self-service)。
package http

import (
	"errors"
	"net/http"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	idmusecases "ra-idp-go/internal/identitymanagement/usecases"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type AccountProfileResponse struct {
	Sub               string                         `json:"sub"`
	PreferredUsername string                         `json:"preferred_username"`
	Name              *string                        `json:"name,omitempty"`
	GivenName         *string                        `json:"given_name,omitempty"`
	FamilyName        *string                        `json:"family_name,omitempty"`
	Email             *string                        `json:"email,omitempty"`
	EmailVerified     bool                           `json:"email_verified"`
	MfaEnrolled       bool                           `json:"mfa_enrolled"`
	Status            spec.UserStatus                `json:"status"`
	Attributes        map[string]spec.AttributeValue `json:"attributes"`
	// ReadableAttributes は self が参照できる属性定義。
	ReadableAttributes []spec.UserAttributeDef `json:"readable_attributes"`
	// EditableAttributes は self が編集できる属性定義 (editable_by_user=true)。
	// UI がフォームを描画するために型・multi_valued 等のメタを併せて返す。
	EditableAttributes []spec.UserAttributeDef `json:"editable_attributes"`
}

// accountSummaryResponse は portal home 用のアカウント概要 (self-service)。
// admin shell 用の AccountContext とは別契約で roles を含めない (wi-21 / ADR-042)。
type accountSummaryResponse struct {
	Sub               string                `json:"sub"`
	PreferredUsername string                `json:"preferred_username"`
	Name              *string               `json:"name,omitempty"`
	Email             *string               `json:"email,omitempty"`
	EmailVerified     bool                  `json:"email_verified"`
	MfaEnrolled       bool                  `json:"mfa_enrolled"`
	Status            spec.UserStatus       `json:"status"`
	LastLoginAt       *time.Time            `json:"last_login_at,omitempty"`
	PasswordChangedAt *time.Time            `json:"password_changed_at,omitempty"`
	RequiredActions   []spec.RequiredAction `json:"required_actions"`
}

type accountProfileUpdateRequest struct {
	Name       *string                         `json:"name"`
	GivenName  *string                         `json:"given_name"`
	FamilyName *string                         `json:"family_name"`
	Attributes *map[string]spec.AttributeValue `json:"attributes"`
}

func toAccountProfileResponse(user *spec.User, defs []spec.UserAttributeDef) AccountProfileResponse {
	return AccountProfileResponse{
		Sub: user.Sub, PreferredUsername: user.PreferredUsername,
		Name: user.Name, GivenName: user.GivenName, FamilyName: user.FamilyName,
		Email: user.Email, EmailVerified: user.EmailVerified, MfaEnrolled: user.MfaEnrolled,
		Status:             user.Lifecycle.EffectiveStatus(),
		Attributes:         idmusecases.SelfReadableAttributes(user.Attributes, defs),
		ReadableAttributes: idmusecases.SelfReadableAttributeDefs(defs),
		EditableAttributes: idmusecases.EditableAttributeDefs(defs),
	}
}

func (d Deps) accountProfileDeps() idmusecases.AccountProfileDeps {
	return idmusecases.AccountProfileDeps{
		UserRepo: d.UserRepo, AttrSchemaRepo: d.AttrSchemaRepo, Emit: d.Emit,
	}
}

func toAccountSummaryResponse(user *spec.User) accountSummaryResponse {
	actions := user.Lifecycle.RequiredActions
	if actions == nil {
		actions = []spec.RequiredAction{}
	}
	return accountSummaryResponse{
		Sub: user.Sub, PreferredUsername: user.PreferredUsername, Name: user.Name,
		Email: user.Email, EmailVerified: user.EmailVerified, MfaEnrolled: user.MfaEnrolled,
		Status:            user.Lifecycle.EffectiveStatus(),
		LastLoginAt:       user.Lifecycle.LastLoginAt,
		PasswordChangedAt: user.Lifecycle.PasswordChangedAt,
		RequiredActions:   actions,
	}
}

func (d Deps) handleGetAccountSummary(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	user, _, err := idmusecases.GetUserProfile(c.Request().Context(), d.accountProfileDeps(), sub)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAccountSummaryResponse(user))
}

func (d Deps) handleGetAccountProfile(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	user, defs, err := idmusecases.GetUserProfile(c.Request().Context(), d.accountProfileDeps(), sub)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAccountProfileResponse(user, defs))
}

func (d Deps) handleUpdateAccountProfile(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input accountProfileUpdateRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	user, defs, err := idmusecases.UpdateUserProfile(c.Request().Context(), d.accountProfileDeps(),
		idmusecases.UpdateUserProfileInput{
			Sub: sub, Name: input.Name, GivenName: input.GivenName, FamilyName: input.FamilyName,
			Attributes: input.Attributes, Now: time.Now().UTC(),
		})
	if err != nil {
		return d.writeAccountError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, toAccountProfileResponse(user, defs))
}

// requireAuthenticatedSub は認証済み (pending でない) セッションの sub を返す。
// self-service では actor == target なので sub をそのまま操作対象に使う。
func (d Deps) requireAuthenticatedSub(c *echo.Context) (string, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return "", err
	}
	if authn == nil || authn.AuthenticationPending {
		return "", core.ErrAdminAuthenticationRequired
	}
	return authn.Sub, nil
}

func (d Deps) writeAccountError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, core.ErrAdminAuthenticationRequired):
		return core.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "認証済みセッションが必要です")
	case errors.Is(err, authusecases.ErrStepUpRequired):
		return core.WriteBrowserError(c, http.StatusForbidden, "step_up_required", "この操作には再認証が必要です")
	case errors.Is(err, idmusecases.ErrUserNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "user_not_found", "ユーザーが存在しません")
	case errors.Is(err, authusecases.ErrSessionNotFound):
		return core.WriteBrowserError(c, http.StatusNotFound, "session_not_found", "セッションが存在しません")
	case errors.Is(err, idmusecases.ErrAttributeNotEditable):
		return core.WriteBrowserError(c, http.StatusForbidden, "attribute_not_editable", "この属性は編集できません")
	case errors.Is(err, idmusecases.ErrInvalidAttribute):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_attribute", "属性がスキーマに適合していません")
	default:
		return err
	}
}

// requireStepUpSub は認証済みセッションを解決し、step-up gate を通過した sub を返す
// (primary email 変更など高 sensitivity な identity 操作用)。
func (d Deps) requireStepUpSub(c *echo.Context) (string, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return "", err
	}
	if authn == nil || authn.AuthenticationPending {
		return "", core.ErrAdminAuthenticationRequired
	}
	if !authusecases.StepUpSatisfied(authn, time.Now().UTC()) {
		return "", authusecases.ErrStepUpRequired
	}
	return authn.Sub, nil
}
