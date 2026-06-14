package http

// SCL interfaces: ListAdminAuditEvents / GetAdminAuditEvent (component: Trust)。
// SCL permission: AdminAuditEventsRead — admin は所属テナント内、system_admin は
// default tenant 経路から全テナント横断で参照できる。書き込み経路は定義しない。

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type adminAuditEventResponse struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	Type       string         `json:"type"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload"`
}

func (d Deps) handleListAdminAuditEvents(c *echo.Context) error {
	actor, err := d.requireAuditReader(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return noStoreJSON(c, http.StatusOK, map[string]any{"events": []adminAuditEventResponse{}})
	}
	query, err := parseAuditEventQuery(c, actor)
	if err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	records, err := d.AuditEventRepo.List(c.Request().Context(), query)
	if err != nil {
		return err
	}
	response := make([]adminAuditEventResponse, len(records))
	for i, rec := range records {
		response[i] = toAdminAuditEventResponse(rec)
	}
	return noStoreJSON(c, http.StatusOK, map[string]any{"events": response})
}

func (d Deps) handleGetAdminAuditEvent(c *echo.Context) error {
	actor, err := d.requireAuditReader(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return writeBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	rec, err := d.AuditEventRepo.FindByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	if rec == nil {
		return writeBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	if !auditEventVisibleTo(rec, actor) {
		// 別テナントのイベントは存在を隠す。
		return writeBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	return noStoreJSON(c, http.StatusOK, toAdminAuditEventResponse(rec))
}

// requireAuditReader は AdminAuditEventsRead パーミッションを満たすユーザーを返す。
// admin / system_admin のどちらでも通る。所属テナントの拘束は問わない (実際の
// テナント絞り込みは List のクエリ生成時に行う)。
func (d Deps) requireAuditReader(c *echo.Context) (*spec.User, error) {
	authn, err := d.resolveAuthentication(c)
	if err != nil {
		return nil, err
	}
	if authn == nil || authn.AuthenticationPending {
		return nil, errAdminAuthenticationRequired
	}
	user, err := d.UserRepo.FindBySub(c.Request().Context(), authn.Sub)
	if err != nil {
		return nil, err
	}
	if user == nil || user.DisabledAt != nil {
		return nil, errAdminAccessDenied
	}
	if !slices.Contains(user.Roles, "admin") && !slices.Contains(user.Roles, "system_admin") {
		return nil, errAdminAccessDenied
	}
	return user, nil
}

func parseAuditEventQuery(c *echo.Context, actor *spec.User) (oauthports.AuditEventQuery, error) {
	q := oauthports.AuditEventQuery{
		TenantID:   actor.TenantID,
		AllTenants: false,
	}
	// system_admin が default tenant 経路で全テナント横断する場合のみ all_tenants を許可する。
	if slices.Contains(actor.Roles, "system_admin") &&
		actor.TenantID == spec.DefaultTenantID &&
		c.QueryParam("all_tenants") == "true" {
		q.AllTenants = true
		q.TenantID = ""
	}
	if t := c.QueryParam("type"); t != "" {
		q.Type = t
	}
	if sub := c.QueryParam("sub"); sub != "" {
		q.Sub = sub
	}
	if after := c.QueryParam("after"); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return oauthports.AuditEventQuery{}, errors.New("after は RFC3339 形式を指定してください")
		}
		q.After = t
	}
	if before := c.QueryParam("before"); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return oauthports.AuditEventQuery{}, errors.New("before は RFC3339 形式を指定してください")
		}
		q.Before = t
	}
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		limit, err := strconv.Atoi(limitParam)
		if err != nil || limit < 0 {
			return oauthports.AuditEventQuery{}, errors.New("limit は 0 以上の整数を指定してください")
		}
		q.Limit = limit
	}
	return q, nil
}

// auditEventVisibleTo は GetAdminAuditEvent で別テナントイベントを隠すための判定。
// system_admin で default テナント在籍なら全件 OK、それ以外は所属テナントのみ。
func auditEventVisibleTo(rec *oauthports.AuditEventRecord, actor *spec.User) bool {
	if slices.Contains(actor.Roles, "system_admin") && actor.TenantID == spec.DefaultTenantID {
		return true
	}
	return rec.TenantID == actor.TenantID
}

func toAdminAuditEventResponse(rec *oauthports.AuditEventRecord) adminAuditEventResponse {
	return adminAuditEventResponse{
		ID:         rec.ID,
		TenantID:   rec.TenantID,
		Type:       rec.Type,
		OccurredAt: rec.OccurredAt,
		Payload:    rec.Payload,
	}
}
