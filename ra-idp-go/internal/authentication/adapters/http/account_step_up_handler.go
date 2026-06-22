// /api/account/step_up — 高 sensitivity な self-service 操作のための step-up 再認証
// (ADR-043 / wi-43)。start は利用可能な factor を返し、complete は password / TOTP を
// 検証して session に step_up_at を刻む。sensitive ハンドラは requireStepUpSub /
// requireStepUpSession を前段ゲートに使い、recency 窓を外れていれば 403 step_up_required。
package http

import (
	"errors"
	"net/http"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

type StepUpStartResponse struct {
	Methods []string `json:"methods"`
}

type stepUpCompleteRequest struct {
	Method   string `json:"method"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (d Deps) stepUpDeps() authusecases.StepUpDeps {
	return authusecases.StepUpDeps{
		UserRepo:       d.UserRepo,
		PasswordHasher: d.PasswordHasher,
		MfaFactorRepo:  d.MfaFactorRepo,
		SessionManager: d.SessionManager,
		Emit:           d.Emit,
	}
}

// requireStepUpSub は認証済みセッションを解決し、step-up gate を通過した sub を返す
// (高 sensitivity 操作用)。recency 窓を外れていれば ErrStepUpRequired。
func (d Deps) requireStepUpSub(c *echo.Context) (string, error) {
	sub, _, err := d.requireStepUpSession(c)
	return sub, err
}

// requireStepUpSession は requireStepUpSub と同じゲートに加え、現在の sessionID を返す
// (revoke_others のように除外対象の session を要するハンドラ用)。
func (d Deps) requireStepUpSession(c *echo.Context) (sub, sessionID string, err error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return "", "", err
	}
	if authn == nil || authn.AuthenticationPending {
		return "", "", core.ErrAdminAuthenticationRequired
	}
	if !authusecases.StepUpSatisfied(authn, time.Now().UTC()) {
		return "", "", authusecases.ErrStepUpRequired
	}
	return authn.Sub, authn.SessionID, nil
}

func (d Deps) handleStartStepUp(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	authn, err := d.requireAuthenticatedAuthn(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	methods, err := authusecases.StepUpStart(c.Request().Context(), d.stepUpDeps(), authn.Sub, authn.SessionID)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	out := make([]string, len(methods))
	for i, m := range methods {
		out[i] = string(m)
	}
	return core.NoStoreJSON(c, http.StatusOK, StepUpStartResponse{Methods: out})
}

func (d Deps) handleCompleteStepUp(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	authn, err := d.requireAuthenticatedAuthn(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input stepUpCompleteRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if err := authusecases.CompleteStepUp(c.Request().Context(), d.stepUpDeps(), authusecases.CompleteStepUpInput{
		Sub:       authn.Sub,
		SessionID: authn.SessionID,
		Method:    authusecases.StepUpMethod(input.Method),
		Password:  input.Password,
		Code:      input.Code,
		Now:       time.Now().UTC(),
	}); err != nil {
		return d.writeStepUpError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

// requireAuthenticatedAuthn は認証済み (pending でない) セッションの AuthenticationContext を
// 返す。step-up start / complete は step-up gate 自体を掛けない (再認証の入口のため)。
func (d Deps) requireAuthenticatedAuthn(c *echo.Context) (*authdomain.AuthenticationContext, error) {
	authn, err := d.ResolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, core.ErrAdminAuthenticationRequired
	}
	return authn, nil
}

func (d Deps) writeStepUpError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, authusecases.ErrStepUpFailed):
		return core.WriteBrowserError(c, http.StatusForbidden, "step_up_failed", "再認証に失敗しました。入力を確認してください。")
	case errors.Is(err, authusecases.ErrStepUpUnsupportedMethod):
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "この再認証方法は利用できません。")
	default:
		return d.writeAccountError(c, err)
	}
}
