package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func TestDiscoveryRoutesIncludeRFC8414Alias(t *testing.T) {
	e := echo.New()
	Register(e, Deps{Issuer: "https://idp.example", SCL: spec.MustLoadSCL()})

	for _, path := range []string{
		"/.well-known/openid-configuration",
		"/.well-known/oauth-authorization-server",
	} {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"acr_values_supported"`)) {
			t.Fatalf("%s omitted acr_values_supported", path)
		}
		// RFC 9207 §3。authorization_response_iss_parameter_supported を必ず広告する。
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"authorization_response_iss_parameter_supported":true`)) {
			t.Fatalf("%s omitted authorization_response_iss_parameter_supported", path)
		}
	}
}
