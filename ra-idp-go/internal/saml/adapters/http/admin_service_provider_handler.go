// SAML 2.0 service provider の管理 API (wi-29)。RequireAdmin で保護し、テナント境界に閉じる。
// entityID は URI (スラッシュを含みうる) のため、取得・削除は path param ではなくボディ/クエリで指定する。
package http

import (
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/infrastructure/http/core"
	samldomain "ra-idp-go/internal/saml/domain"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type serviceProviderRequest struct {
	EntityID                string                  `json:"entity_id"`
	DisplayName             string                  `json:"display_name"`
	ACSURLs                 []string                `json:"acs_urls"`
	SLOURL                  string                  `json:"slo_url"`
	Audience                string                  `json:"audience"`
	ClaimPolicy             spec.ClaimMappingPolicy `json:"claim_policy"`
	SignAssertion           *bool                   `json:"sign_assertion"`
	SignResponse            bool                    `json:"sign_response"`
	WantAuthnRequestsSigned bool                    `json:"want_authn_requests_signed"`
	SigningCertificatePEM   string                  `json:"authn_request_signing_certificate_pem"`
}

func (r serviceProviderRequest) validate() error {
	switch {
	case strings.TrimSpace(r.EntityID) == "":
		return errBadRequest("entity_id is required")
	case len(r.ACSURLs) == 0:
		return errBadRequest("acs_urls must not be empty")
	case strings.TrimSpace(r.ClaimPolicy.NameID.Format) == "":
		return errBadRequest("claim_policy.name_id.format is required")
	case strings.TrimSpace(r.ClaimPolicy.NameID.SourceAttribute) == "":
		return errBadRequest("claim_policy.name_id.source_attribute is required")
	}
	if r.WantAuthnRequestsSigned {
		if _, err := samldomain.ParseCertificatePEM(r.SigningCertificatePEM); err != nil {
			return errBadRequest("authn_request_signing_certificate_pem is required when want_authn_requests_signed is true")
		}
	}
	return nil
}

type badRequest struct{ msg string }

func (e badRequest) Error() string   { return e.msg }
func errBadRequest(msg string) error { return badRequest{msg: msg} }

func (d Deps) handleListServiceProviders(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.SamlSPRepo == nil {
		return core.NoStoreJSON(c, http.StatusOK, map[string]any{"service_providers": []any{}})
	}
	sps, err := d.SamlSPRepo.ListByTenant(c.Request().Context(), core.RequestTenantID(c))
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"service_providers": sps})
}

func (d Deps) handleUpsertServiceProvider(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.SamlSPRepo == nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "SAML は利用できません")
	}
	var req serviceProviderRequest
	if err := c.Bind(&req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSON が不正です")
	}
	if err := req.validate(); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}

	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)
	now := time.Now().UTC()

	existing, err := d.SamlSPRepo.FindByEntityID(ctx, tenantID, req.EntityID)
	if err != nil {
		return err
	}
	signAssertion := true
	if req.SignAssertion != nil {
		signAssertion = *req.SignAssertion
	}
	sp := &spec.SamlServiceProvider{
		TenantID:                          tenantID,
		EntityID:                          req.EntityID,
		DisplayName:                       req.DisplayName,
		ACSURLs:                           req.ACSURLs,
		SLOURL:                            strings.TrimSpace(req.SLOURL),
		Audience:                          strings.TrimSpace(req.Audience),
		ClaimPolicy:                       req.ClaimPolicy,
		SignAssertion:                     signAssertion,
		SignResponse:                      req.SignResponse,
		WantAuthnRequestsSigned:           req.WantAuthnRequestsSigned,
		AuthnRequestSigningCertificatePEM: strings.TrimSpace(req.SigningCertificatePEM),
		CreatedAt:                         now,
	}
	status := http.StatusCreated
	if existing != nil {
		sp.CreatedAt = existing.CreatedAt
		sp.UpdatedAt = &now
		status = http.StatusOK
	}
	if err := d.SamlSPRepo.Save(ctx, sp); err != nil {
		return err
	}
	return core.NoStoreJSON(c, status, sp)
}

func (d Deps) handleDeleteServiceProvider(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.SamlSPRepo == nil {
		return c.NoContent(http.StatusNoContent)
	}
	entityID := strings.TrimSpace(c.QueryParam("entity_id"))
	if entityID == "" {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "entity_id query parameter is required")
	}
	if err := d.SamlSPRepo.Delete(c.Request().Context(), core.RequestTenantID(c), entityID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
