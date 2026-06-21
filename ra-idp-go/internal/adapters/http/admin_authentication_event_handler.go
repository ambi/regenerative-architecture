package http

// SCL interfaces: ListAuthenticationEvents / GetAuthenticationEvent /
// ExportAuthenticationEvents (component: Authentication, wi-44)。
// SCL permission: AdminAuthenticationEventsRead — admin は所属テナント内、system_admin は
// default tenant 経路から全テナント横断で参照できる (requireAuditReader と同じ述語)。
//
// 認証イベントは監査イベントストア (AuditEventRepository) に蓄積した DomainEvent を
// success / fail / aggregated の type 群へ絞って読み出す射影である (signin_activity と同じ
// grain)。ADR-045 に従い admin 検索は必ず期間 (from / to) の絞り込みを要求する。

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// 認証イベントの kind (SCL AuthenticationEventKind) → 監査 type 群。
var authenticationEventKindTypes = map[string][]string{
	"success": {
		(&spec.UserAuthenticated{}).EventType(),
		(&spec.AuthenticationStepCompleted{}).EventType(),
		(&spec.MfaChallengeIssued{}).EventType(),
		(&spec.MfaChallengeSucceeded{}).EventType(),
		(&spec.BackupCodeConsumed{}).EventType(),
		(&spec.SessionStarted{}).EventType(),
		(&spec.SessionRefreshed{}).EventType(),
		(&spec.SessionEnded{}).EventType(),
		(&spec.FederatedAuthenticated{}).EventType(),
		(&spec.FederationLinked{}).EventType(),
		(&spec.FederationUnlinked{}).EventType(),
		(&spec.SessionImpersonationStarted{}).EventType(),
		(&spec.SessionImpersonationEnded{}).EventType(),
	},
	"fail": {
		(&spec.AuthenticationFailed{}).EventType(),
		(&spec.AuthenticationStepFailed{}).EventType(),
		(&spec.MfaChallengeFailed{}).EventType(),
	},
	"aggregated": {
		(&spec.AuthenticationEventAggregated{}).EventType(),
		(&spec.LoginThrottled{}).EventType(),
	},
}

// allAuthenticationEventTypes は kind 未指定時に対象とする全 type。
var allAuthenticationEventTypes = func() []string {
	out := []string{}
	for _, kind := range []string{"success", "fail", "aggregated"} {
		out = append(out, authenticationEventKindTypes[kind]...)
	}
	return out
}()

const authenticationEventExportMaxLimit = 10000

func (d Deps) handleListAuthenticationEvents(c *echo.Context) error {
	actor, err := d.requireAuthenticationEventsReader(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return noStoreJSON(c, http.StatusOK, map[string]any{"events": []adminAuditEventResponse{}})
	}
	query, err := parseAuthenticationEventQuery(c, actor, false)
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

func (d Deps) handleGetAuthenticationEvent(c *echo.Context) error {
	actor, err := d.requireAuthenticationEventsReader(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return writeBrowserError(c, http.StatusNotFound, "event_not_found", "認証イベントが存在しません")
	}
	rec, err := d.AuditEventRepo.FindByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	if rec == nil || !slices.Contains(allAuthenticationEventTypes, rec.Type) || !auditEventVisibleTo(rec, actor) {
		// 認証イベント以外・別テナントは存在を隠す。
		return writeBrowserError(c, http.StatusNotFound, "event_not_found", "認証イベントが存在しません")
	}
	return noStoreJSON(c, http.StatusOK, toAdminAuditEventResponse(rec))
}

func (d Deps) handleExportAuthenticationEvents(c *echo.Context) error {
	actor, err := d.requireAuthenticationEventsReader(c)
	if err != nil {
		return d.writeAdminAccessError(c, err)
	}
	query, err := parseAuthenticationEventQuery(c, actor, true)
	if err != nil {
		return writeBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	var records []*oauthports.AuditEventRecord
	if d.AuditEventRepo != nil {
		records, err = d.AuditEventRepo.List(c.Request().Context(), query)
		if err != nil {
			return err
		}
	}
	response := make([]adminAuditEventResponse, len(records))
	for i, rec := range records {
		response[i] = toAdminAuditEventResponse(rec)
	}
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"authentication_events.json\"")
	return noStoreJSON(c, http.StatusOK, map[string]any{"events": response})
}

// requireAuthenticationEventsReader は AdminAuthenticationEventsRead を満たすユーザーを返す。
// allow_when は AdminAuditEventsRead と同一述語 (admin / system_admin) なので判定を共有する。
func (d Deps) requireAuthenticationEventsReader(c *echo.Context) (*spec.User, error) {
	return d.requireAuditReader(c)
}

func parseAuthenticationEventQuery(c *echo.Context, actor *spec.User, export bool) (oauthports.AuditEventQuery, error) {
	q := oauthports.AuditEventQuery{TenantID: actor.TenantID}
	if slices.Contains(actor.Roles, "system_admin") &&
		actor.TenantID == spec.DefaultTenantID &&
		c.QueryParam("all_tenants") == "true" {
		q.AllTenants = true
		q.TenantID = ""
	}

	// ADR-045: admin 検索は必ず期間絞り込みを要求し、全期間スキャンを禁じる。
	from, err := parseRequiredRFC3339(c.QueryParam("from"), "from")
	if err != nil {
		return oauthports.AuditEventQuery{}, err
	}
	to, err := parseRequiredRFC3339(c.QueryParam("to"), "to")
	if err != nil {
		return oauthports.AuditEventQuery{}, err
	}
	if !to.After(from) {
		return oauthports.AuditEventQuery{}, errors.New("to は from より後を指定してください")
	}
	q.After, q.Before = from, to

	if kind := c.QueryParam("kind"); kind != "" {
		types, ok := authenticationEventKindTypes[kind]
		if !ok {
			return oauthports.AuditEventQuery{}, errors.New("kind は success / fail / aggregated を指定してください")
		}
		q.Types = types
	} else {
		q.Types = allAuthenticationEventTypes
	}

	if sub := strings.TrimSpace(c.QueryParam("sub")); sub != "" {
		q.Sub = sub
	}
	payloadEquals := map[string]string{}
	if v := strings.TrimSpace(c.QueryParam("username_hash")); v != "" {
		payloadEquals["usernameHash"] = v
	}
	if v := strings.TrimSpace(c.QueryParam("ip_truncated")); v != "" {
		payloadEquals["ipTruncated"] = v
	}
	if len(payloadEquals) > 0 {
		q.PayloadEquals = payloadEquals
	}

	if limitParam := c.QueryParam("limit"); limitParam != "" {
		limit, err := strconv.Atoi(limitParam)
		if err != nil || limit < 0 {
			return oauthports.AuditEventQuery{}, errors.New("limit は 0 以上の整数を指定してください")
		}
		q.Limit = limit
	}
	if export {
		q.Limit = authenticationEventExportMaxLimit
	}
	return q, nil
}

func parseRequiredRFC3339(value, field string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New(field + " は必須です (RFC3339)")
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New(field + " は RFC3339 形式を指定してください")
	}
	return t, nil
}
