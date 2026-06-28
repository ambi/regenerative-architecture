// /api/account/security — エンドユーザー自身のセキュリティ概要と MFA (TOTP) の
// self-service 登録・解除 (wi-21 / ADR-042)。登録は確認コード、解除は有効な TOTP コードに
// よる所持証明に加え、step-up 再認証 (ADR-043) を要求する。
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

type accountMfaFactorResponse struct {
	Type       spec.MfaFactorType `json:"type"`
	Label      *string            `json:"label,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	LastUsedAt *time.Time         `json:"last_used_at,omitempty"`
}

type accountSecurityResponse struct {
	PasswordChangedAt *time.Time                 `json:"password_changed_at,omitempty"`
	TotpEnrolled      bool                       `json:"totp_enrolled"`
	Factors           []accountMfaFactorResponse `json:"factors"`
}

type totpEnrollmentStartResponse struct {
	Secret      string `json:"secret"`
	OTPAuthURI  string `json:"otpauth_uri"`
	AccountName string `json:"account_name"`
	Issuer      string `json:"issuer"`
}

type totpEnrollmentConfirmRequest struct {
	Secret string `json:"secret"`
	Code   string `json:"code"`
}

type mfaFactorRemoveRequest struct {
	Code string `json:"code"`
}

func (d Deps) accountMfaDeps() authusecases.AccountMfaDeps {
	return authusecases.AccountMfaDeps{
		UserRepo: d.UserRepo, MfaFactorRepo: d.MfaFactorRepo, Emit: d.Emit, Issuer: d.Issuer,
	}
}

func (d Deps) handleGetAccountSecurity(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	user, _, err := idmusecases.GetUserProfile(c.Request().Context(), d.accountProfileDeps(), sub)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	factors, err := d.MfaFactorRepo.ListBySub(c.Request().Context(), sub)
	if err != nil {
		return err
	}
	responses := make([]accountMfaFactorResponse, 0, len(factors))
	totpEnrolled := false
	for _, factor := range factors {
		if factor.Type == spec.MfaFactorTOTP {
			totpEnrolled = true
		}
		responses = append(responses, accountMfaFactorResponse{
			Type: factor.Type, Label: factor.Label,
			CreatedAt: factor.CreatedAt, LastUsedAt: factor.LastUsedAt,
		})
	}
	return core.NoStoreJSON(c, http.StatusOK, accountSecurityResponse{
		PasswordChangedAt: user.Lifecycle.PasswordChangedAt,
		TotpEnrolled:      totpEnrolled,
		Factors:           responses,
	})
}

func (d Deps) handleStartTotpEnrollment(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	start, err := authusecases.StartTOTPEnrollment(c.Request().Context(), d.accountMfaDeps(), sub)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	return core.NoStoreJSON(c, http.StatusOK, totpEnrollmentStartResponse{
		Secret: start.Secret, OTPAuthURI: start.OTPAuthURI,
		AccountName: start.AccountName, Issuer: start.Issuer,
	})
}

func (d Deps) handleConfirmTotpEnrollment(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input totpEnrollmentConfirmRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if err := authusecases.ConfirmTOTPEnrollment(c.Request().Context(), d.accountMfaDeps(),
		authusecases.ConfirmTOTPEnrollmentInput{
			Sub: sub, Secret: input.Secret, Code: input.Code, Now: time.Now().UTC(),
		}); err != nil {
		return d.writeAccountError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (d Deps) handleRemoveTotpFactor(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	// MFA factor の解除は高 sensitivity 操作。step-up 再認証を要求する (ADR-043)。
	sub, err := d.requireStepUpSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	var input mfaFactorRemoveRequest
	if err := core.DecodeJSON(c.Request(), &input); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if err := authusecases.RemoveTOTPFactor(c.Request().Context(), d.accountMfaDeps(),
		authusecases.RemoveTOTPFactorInput{Sub: sub, Code: input.Code, Now: time.Now().UTC()}); err != nil {
		return d.writeAccountError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

// writeAccountMfaError は MFA self-service 固有のエラーを HTTP に写す。該当した場合は
// handled=true と書き込み結果を返し、それ以外は handled=false で writeAccountError に委ねる。
func writeAccountMfaError(c *echo.Context, err error) (handled bool, result error) {
	switch {
	case errors.Is(err, authusecases.ErrMfaAlreadyEnrolled):
		return true, core.WriteBrowserError(c, http.StatusConflict, "mfa_already_enrolled", "認証アプリは既に登録されています")
	case errors.Is(err, authusecases.ErrMfaNotEnrolled):
		return true, core.WriteBrowserError(c, http.StatusNotFound, "mfa_not_enrolled", "登録済みの認証アプリがありません")
	case errors.Is(err, authusecases.ErrInvalidTOTPCode):
		return true, core.WriteBrowserError(c, http.StatusBadRequest, "invalid_totp", "認証コードを確認してください。")
	case errors.Is(err, authusecases.ErrInvalidTOTPSecret):
		return true, core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "登録手続きをやり直してください。")
	default:
		return false, nil
	}
}
