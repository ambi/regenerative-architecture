package http

import (
	"html/template"
	"net/http"

	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>ログイン</title></head>
<body>
<h1>ログインが必要です</h1>
<form method="POST" action="/login">
  <input type="hidden" name="request_id" value="{{.RequestID}}">
  <label>ユーザー名 <input name="username" autocomplete="username" required></label>
  <label>パスワード <input name="password" type="password" autocomplete="current-password" required></label>
  <button type="submit">ログイン</button>
</form>
</body></html>`))

var consentTmpl = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>同意</title></head>
<body>
<h1>{{.ClientName}} がアクセスを要求しています</h1>
<p>要求スコープ: <code>{{.Scope}}</code></p>
<form method="POST" action="/consent">
  <input type="hidden" name="request_id" value="{{.RequestID}}">
  <button type="submit" name="action" value="allow">許可</button>
  <button type="submit" name="action" value="deny">拒否</button>
</form>
</body></html>`))

func renderLogin(c *echo.Context, requestID string) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
	c.Response().WriteHeader(http.StatusUnauthorized)
	return loginTmpl.Execute(c.Response(), struct{ RequestID string }{RequestID: requestID})
}

func renderConsent(c *echo.Context, req *spec.AuthorizationRequest, client *spec.Client) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=UTF-8")
	c.Response().WriteHeader(http.StatusOK)
	name := client.ClientID
	if client.ClientName != nil {
		name = *client.ClientName
	}
	return consentTmpl.Execute(c.Response(), struct {
		ClientName, Scope, RequestID string
	}{ClientName: name, Scope: req.Scope, RequestID: req.ID})
}
