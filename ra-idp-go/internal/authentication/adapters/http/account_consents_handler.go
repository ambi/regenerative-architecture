// /api/account/consents — エンドユーザー自身の接続済みアプリ (Consent) の参照と取り消し
// (self-service, wi-21)。
package http

import (
	"net/http"
	"slices"
	"time"

	"ra-idp-go/internal/infrastructure/http/core"
	oauthusecases "ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type accountConsentResponse struct {
	ClientID  string            `json:"client_id"`
	Scopes    []string          `json:"scopes"`
	State     spec.ConsentState `json:"state"`
	GrantedAt time.Time         `json:"granted_at"`
	ExpiresAt time.Time         `json:"expires_at"`
}

func toAccountConsentResponse(consent *spec.Consent) accountConsentResponse {
	return accountConsentResponse{
		ClientID: consent.ClientID, Scopes: slices.Clone(consent.Scopes), State: consent.State,
		GrantedAt: consent.GrantedAt, ExpiresAt: consent.ExpiresAt,
	}
}

func (d Deps) handleListAccountConsents(c *echo.Context) error {
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	consents, err := oauthusecases.ListConsentsForSub(c.Request().Context(), d.ConsentDeps(), sub)
	if err != nil {
		return err
	}
	response := make([]accountConsentResponse, len(consents))
	for i, consent := range consents {
		response[i] = toAccountConsentResponse(consent)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"consents": response})
}

func (d Deps) handleRevokeAccountConsent(c *echo.Context) error {
	if err := d.VerifyBrowserRequest(c); err != nil {
		return err
	}
	sub, err := d.requireAuthenticatedSub(c)
	if err != nil {
		return d.writeAccountError(c, err)
	}
	// actor も target も自分の sub に固定する。URL の client_id 以外は信用しない。
	if err := oauthusecases.RevokeConsent(
		c.Request().Context(), d.ConsentDeps(), sub, sub, c.Param("client_id"), time.Now().UTC(),
	); err != nil {
		return d.WriteConsentError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}
