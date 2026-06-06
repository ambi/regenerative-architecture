package http

import (
	"errors"
	"net/http"

	"ra-idp-go/internal/oauth2/usecases"

	"github.com/labstack/echo/v5"
)

func writeOAuthError(c *echo.Context, err error) error {
	var oe *usecases.OAuthError
	if !errors.As(err, &oe) {
		return c.JSON(http.StatusInternalServerError, oauthErrorBody("server_error", err.Error()))
	}
	status := http.StatusBadRequest
	switch oe.Code {
	case "invalid_client":
		status = http.StatusUnauthorized
	case "server_error":
		status = http.StatusInternalServerError
	}
	return c.JSON(status, oauthErrorBody(oe.Code, oe.Description))
}

func oauthErrorBody(code, description string) map[string]string {
	return map[string]string{"error": code, "error_description": description}
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
