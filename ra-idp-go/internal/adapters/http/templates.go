package http

import (
	"encoding/json"
	"html/template"
	"net/http"

	"ra-idp-go/internal/spec"
	"ra-idp-go/ui"

	"github.com/labstack/echo/v5"
)

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>{{.Title}} | RA Identity</title>
  <link rel="stylesheet" href="/ui/assets/app.css">
</head>
<body>
  <div id="root"></div>
  <script id="ra-page-data" type="application/json">{{.PageData}}</script>
  <script type="module" src="/ui/assets/app.js"></script>
</body>
</html>`))

type loginPage struct {
	Kind      string `json:"kind"`
	RequestID string `json:"requestId"`
	Error     string `json:"error,omitempty"`
}

type consentPage struct {
	Kind       string `json:"kind"`
	RequestID  string `json:"requestId"`
	ClientName string `json:"clientName"`
	Scope      string `json:"scope"`
}

type devicePage struct {
	Kind     string `json:"kind"`
	UserCode string `json:"userCode"`
}

type statusPage struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

func renderLogin(c *echo.Context, requestID, message string) error {
	return renderPage(c, http.StatusUnauthorized, "ログイン", loginPage{
		Kind: "login", RequestID: requestID, Error: message,
	})
}

func renderConsent(c *echo.Context, req *spec.AuthorizationRequest, client *spec.Client) error {
	name := client.ClientID
	if client.ClientName != nil {
		name = *client.ClientName
	}
	return renderPage(c, http.StatusOK, "アクセスの確認", consentPage{
		Kind: "consent", RequestID: req.ID, ClientName: name, Scope: req.Scope,
	})
}

func renderDevice(c *echo.Context, userCode string) error {
	return renderPage(c, http.StatusOK, "デバイスを接続", devicePage{
		Kind: "device", UserCode: userCode,
	})
}

func renderStatus(c *echo.Context, statusCode int, status string) error {
	return renderPage(c, statusCode, "RA Identity", statusPage{
		Kind: "status", Status: status,
	})
}

func renderPage(c *echo.Context, status int, title string, data any) error {
	pageData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	response := c.Response()
	response.Header().Set("Content-Type", "text/html; charset=UTF-8")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "DENY")
	response.WriteHeader(status)
	return pageTmpl.Execute(response, struct {
		Title    string
		PageData template.JS
	}{
		Title:    title,
		PageData: template.JS(pageData), //nolint:gosec // encoding/json escapes HTML-significant bytes.
	})
}

func serveUIAsset(c *echo.Context, name, contentType string) error {
	content, err := ui.ReadAsset(name)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.Blob(http.StatusOK, contentType, content)
}
