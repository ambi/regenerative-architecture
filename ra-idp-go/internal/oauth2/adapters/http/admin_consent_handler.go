package http

import (
	"net/http"
	"slices"
	"time"

	oauthusecases "ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

type adminConsentResponse struct {
	TenantID  string            `json:"tenant_id"`
	Sub       string            `json:"sub"`
	ClientID  string            `json:"client_id"`
	Scopes    []string          `json:"scopes"`
	State     spec.ConsentState `json:"state"`
	GrantedAt time.Time         `json:"granted_at"`
	ExpiresAt time.Time         `json:"expires_at"`
	RevokedAt *time.Time        `json:"revoked_at,omitempty"`
}

func (d Deps) handleListAdminConsents(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	consents, err := oauthusecases.ListConsents(c.Request().Context(), d.ConsentDeps())
	if err != nil {
		return err
	}
	response := make([]adminConsentResponse, len(consents))
	for i, consent := range consents {
		response[i] = toAdminConsentResponse(consent)
	}
	return support.NoStoreJSON(c, http.StatusOK, map[string]any{"consents": response})
}

func (d Deps) handleGetAdminConsent(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	consent, err := oauthusecases.GetConsent(
		c.Request().Context(), d.ConsentDeps(), c.Param("sub"), c.Param("client_id"),
	)
	if err != nil {
		return d.WriteConsentError(c, err)
	}
	return support.NoStoreJSON(c, http.StatusOK, toAdminConsentResponse(consent))
}

func (d Deps) handleRevokeAdminConsent(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	actor, err := d.RequireAdmin(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if err := oauthusecases.RevokeConsent(
		c.Request().Context(), d.ConsentDeps(), actor.Sub,
		c.Param("sub"), c.Param("client_id"), time.Now().UTC(),
	); err != nil {
		return d.WriteConsentError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func toAdminConsentResponse(consent *spec.Consent) adminConsentResponse {
	return adminConsentResponse{
		TenantID: consent.TenantID, Sub: consent.Sub, ClientID: consent.ClientID,
		Scopes: slices.Clone(consent.Scopes), State: consent.State,
		GrantedAt: consent.GrantedAt, ExpiresAt: consent.ExpiresAt, RevokedAt: consent.RevokedAt,
	}
}
