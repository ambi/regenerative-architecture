// /device_authorization + browser device approval API
package http

import (
	"net/http"
	"strings"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/shared/adapters/http/support"

	"github.com/labstack/echo/v5"
)

type deviceAPIRequest struct {
	UserCode string `json:"user_code"`
	Action   string `json:"action"`
}

func (d Deps) handleDeviceAuthorization(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, support.OAuthErrorBody("invalid_request", "form parse"))
	}
	client, err := d.authenticateTokenClient(c)
	if err != nil {
		return writeOAuthError(c, err)
	}
	in := usecases.DeviceAuthorizationInput{
		ClientID: client.ID,
		Scope:    c.Request().PostFormValue("scope"),
	}
	res, err := usecases.RequestDeviceAuthorization(c.Request().Context(), usecases.DeviceAuthorizationDeps{
		ClientRepo: d.ClientRepo, DeviceCodeStore: d.DeviceCodeStore,
		BaseVerification: support.RequestIssuer(c, d.Issuer) + "/device", Emit: d.Emit,
	}, in, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (d Deps) handleDeviceContext(c *echo.Context) error {
	csrf, err := d.EnsureCSRFCookie(c)
	if err != nil {
		return err
	}
	userCode := strings.ToUpper(strings.TrimSpace(c.QueryParam("user_code")))
	return support.NoStoreJSON(c, http.StatusOK, map[string]string{
		"kind": "device", "user_code": userCode, "csrf_token": csrf,
	})
}

func (d Deps) handleDeviceAPI(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	var input deviceAPIRequest
	if err := support.DecodeJSON(c.Request(), &input); err != nil {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSONリクエストが不正です")
	}
	if strings.TrimSpace(input.UserCode) == "" {
		return support.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "デバイスコードが必要です")
	}
	authn, _ := d.AuthnResolver.Resolve(
		c.Request().Context(),
		authdomain.HTTPHeadersAdapter{H: c.Request().Header},
	)
	if authn == nil {
		return support.WriteBrowserError(c, http.StatusUnauthorized, "authentication_required", "ログインが必要です")
	}
	if input.Action == "deny" {
		if err := usecases.DenyUserCode(
			c.Request().Context(),
			usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit},
			input.UserCode,
			authn.Sub,
			time.Now().UTC(),
		); err != nil {
			return writeOAuthError(c, err)
		}
		return support.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: "/status?state=denied"})
	}
	if err := usecases.ApproveUserCode(
		c.Request().Context(),
		usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit},
		input.UserCode,
		authn.Sub,
		time.Now().UTC(),
	); err != nil {
		return writeOAuthError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, browserFlowResponse{Next: "/status?state=approved"})
}
