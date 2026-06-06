// /device_authorization + /device (user code entry / approval UI)
package http

import (
	"html/template"
	"net/http"
	"time"

	authdomain "ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/oauth2/usecases"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleDeviceAuthorization(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return c.JSON(http.StatusBadRequest, oauthErrorBody("invalid_request", "form parse"))
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
		BaseVerification: d.Issuer + "/device", Emit: d.Emit,
	}, in, time.Now().UTC())
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

func (d Deps) handleDeviceVerification(c *echo.Context) error {
	if c.Request().Method == http.MethodGet {
		return c.HTML(http.StatusOK, deviceVerificationForm(c.QueryParam("user_code")))
	}
	if err := c.Request().ParseForm(); err != nil {
		return c.String(http.StatusBadRequest, "invalid form")
	}
	userCode := c.Request().PostFormValue("user_code")
	action := c.Request().PostFormValue("action")
	authn, _ := d.AuthnResolver.Resolve(c.Request().Context(), authdomain.HTTPHeadersAdapter{H: c.Request().Header})
	if authn == nil {
		return c.HTML(http.StatusUnauthorized, "<!doctype html><h1>ログインが必要です</h1>")
	}
	if action == "deny" {
		_ = usecases.DenyUserCode(c.Request().Context(), usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit}, userCode, authn.Sub, time.Now().UTC())
		return c.HTML(http.StatusOK, "<!doctype html><h1>拒否しました</h1>")
	}
	if err := usecases.ApproveUserCode(c.Request().Context(), usecases.VerifyUserCodeDeps{DeviceCodeStore: d.DeviceCodeStore, Emit: d.Emit}, userCode, authn.Sub, time.Now().UTC()); err != nil {
		return writeOAuthError(c, err)
	}
	return c.HTML(http.StatusOK, "<!doctype html><h1>承認しました</h1>")
}

func deviceVerificationForm(userCode string) string {
	return `<!doctype html><html lang="ja"><head><meta charset="utf-8"><title>デバイス承認</title></head>
<body>
<h1>デバイス承認</h1>
<form method="POST" action="/device">
  <label>ユーザーコード <input name="user_code" value="` + template.HTMLEscapeString(userCode) + `" required></label>
  <button type="submit" name="action" value="allow">承認</button>
  <button type="submit" name="action" value="deny">拒否</button>
</form>
</body></html>`
}
