// WS-Federation relying party の管理 API (wi-61)。RequireAdmin で保護し、テナント境界に閉じる。
// wtrealm は URI (スラッシュを含みうる) のため、取得・削除は path param ではなくボディ/クエリで指定する。
package http

import (
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type relyingPartyRequest struct {
	Wtrealm     string                  `json:"wtrealm"`
	DisplayName string                  `json:"display_name"`
	ReplyURLs   []string                `json:"reply_urls"`
	Audience    string                  `json:"audience"`
	ClaimPolicy spec.ClaimMappingPolicy `json:"claim_policy"`
}

func (r relyingPartyRequest) validate() error {
	switch {
	case strings.TrimSpace(r.Wtrealm) == "":
		return errBadRequest("wtrealm is required")
	case len(r.ReplyURLs) == 0:
		return errBadRequest("reply_urls must not be empty")
	case strings.TrimSpace(r.ClaimPolicy.NameID.Format) == "":
		return errBadRequest("claim_policy.name_id.format is required")
	case strings.TrimSpace(r.ClaimPolicy.NameID.SourceAttribute) == "":
		return errBadRequest("claim_policy.name_id.source_attribute is required")
	}
	return nil
}

type badRequest struct{ msg string }

func (e badRequest) Error() string   { return e.msg }
func errBadRequest(msg string) error { return badRequest{msg: msg} }

func (d Deps) handleListRelyingParties(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	parts, err := d.WsFedRPRepo.ListByTenant(c.Request().Context(), core.RequestTenantID(c))
	if err != nil {
		return err
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"relying_parties": parts})
}

func (d Deps) handleUpsertRelyingParty(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	var req relyingPartyRequest
	if err := c.Bind(&req); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "JSON が不正です")
	}
	if err := req.validate(); err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}

	ctx := c.Request().Context()
	tenantID := core.RequestTenantID(c)
	now := time.Now().UTC()

	existing, err := d.WsFedRPRepo.FindByWtrealm(ctx, tenantID, req.Wtrealm)
	if err != nil {
		return err
	}
	rp := &spec.WsFedRelyingParty{
		TenantID:    tenantID,
		Wtrealm:     req.Wtrealm,
		DisplayName: req.DisplayName,
		ReplyURLs:   req.ReplyURLs,
		Audience:    req.Audience,
		ClaimPolicy: req.ClaimPolicy,
		CreatedAt:   now,
	}
	status := http.StatusCreated
	if existing != nil {
		rp.CreatedAt = existing.CreatedAt
		rp.UpdatedAt = &now
		status = http.StatusOK
	}
	if err := d.WsFedRPRepo.Save(ctx, rp); err != nil {
		return err
	}
	return core.NoStoreJSON(c, status, rp)
}

func (d Deps) handleDeleteRelyingParty(c *echo.Context) error {
	if _, err := d.RequireAdmin(c); err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	wtrealm := strings.TrimSpace(c.QueryParam("wtrealm"))
	if wtrealm == "" {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", "wtrealm query parameter is required")
	}
	if err := d.WsFedRPRepo.Delete(c.Request().Context(), core.RequestTenantID(c), wtrealm); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
