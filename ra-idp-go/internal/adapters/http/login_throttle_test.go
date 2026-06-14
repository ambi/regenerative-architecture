package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestExtractClientIPUsesOnlyTrustedForwardedHops(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 198.51.100.20")
	if got := extractClientIP(req, 0); got != "" {
		t.Fatalf("trustedHops=0 returned %q", got)
	}
	if got := extractClientIP(req, 1); got != "203.0.113.10" {
		t.Fatalf("trustedHops=1 returned %q", got)
	}
	if got := extractClientIP(req, 2); got != "" {
		t.Fatalf("trustedHops=2 returned %q", got)
	}
}

func TestWriteLoginThrottledReturnsRetryAfter(t *testing.T) {
	e := echo.New()
	e.POST("/login", func(c *echo.Context) error {
		return writeLoginThrottled(c, 900)
	})
	req := httptest.NewRequest(http.MethodPost, "/login", http.NoBody)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "900" {
		t.Fatalf("Retry-After=%q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q", got)
	}
}
