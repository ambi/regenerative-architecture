package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	authdomain "ra-idp-go/internal/authentication/domain"

	"github.com/labstack/echo/v5"
)

type fakeAuthnResolver struct {
	ctx *authdomain.AuthenticationContext
}

func (f *fakeAuthnResolver) Resolve(_ context.Context, _ authdomain.Headers) (*authdomain.AuthenticationContext, error) {
	return f.ctx, nil
}

type testingT interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

func passwordResetContextCSRF(t testingT, e *echo.Echo, path string) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("csrf context status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("csrf cookie missing")
	}
	return body.CSRFToken, cookies[0]
}
