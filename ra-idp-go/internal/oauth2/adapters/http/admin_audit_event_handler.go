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
	"ra-idp-go/internal/platform/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type AdminAuditEventResponse struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	Type       string         `json:"type"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload"`
}

// 監査ログのイベントカテゴリ → 監査 type 群 (wi-44)。admin が分かりやすく絞り込めるよう、
// 認証系は成功 / 失敗 / 集約のサブ分類を持ち (authentication はその和集合)、管理操作系も
// 大分類でまとめる。type 完全一致 (query.type) は機械向けの低レベルフィルタとして別に残す。
// 各値は SCL events の EventType 文字列 (owns_events と一致)。
var auditEventCategoryTypes = map[string][]string{
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
	"user": {
		(&spec.UserCreated{}).EventType(),
		(&spec.UserUpdated{}).EventType(),
		(&spec.UserDisabled{}).EventType(),
		(&spec.UserEnabled{}).EventType(),
		(&spec.UserDeleted{}).EventType(),
		(&spec.UserRequiredActionSet{}).EventType(),
		(&spec.UserRequiredActionCleared{}).EventType(),
		(&spec.PasswordChanged{}).EventType(),
		(&spec.PasswordResetRequested{}).EventType(),
		(&spec.EmailChangeRequested{}).EventType(),
		(&spec.EmailChanged{}).EventType(),
		(&spec.MfaFactorEnrolled{}).EventType(),
		(&spec.MfaFactorRemoved{}).EventType(),
	},
	"group": {
		(&spec.GroupCreated{}).EventType(),
		(&spec.GroupUpdated{}).EventType(),
		(&spec.GroupDeleted{}).EventType(),
		(&spec.GroupMemberAdded{}).EventType(),
		(&spec.GroupMemberRemoved{}).EventType(),
	},
	"client": {
		(&spec.ClientRegistered{}).EventType(),
		(&spec.AdminClientCreated{}).EventType(),
		(&spec.AdminClientUpdated{}).EventType(),
		(&spec.AdminClientDeleted{}).EventType(),
	},
	"consent": {
		(&spec.ConsentGrantedEvent{}).EventType(),
		(&spec.ConsentRevokedEvent{}).EventType(),
	},
	"token": {
		(&spec.AuthorizationCodeIssued{}).EventType(),
		(&spec.AuthorizationCodeRedeemed{}).EventType(),
		(&spec.AccessTokenIssued{}).EventType(),
		(&spec.RefreshTokenIssued{}).EventType(),
		(&spec.RefreshTokenRotated{}).EventType(),
		(&spec.TokenRevoked{}).EventType(),
		(&spec.TokenIntrospected{}).EventType(),
		(&spec.RefreshTokenReuseDetected{}).EventType(),
		(&spec.PARStored{}).EventType(),
		(&spec.DeviceAuthorizationRequested{}).EventType(),
		(&spec.DeviceAuthorizationApproved{}).EventType(),
		(&spec.DeviceAuthorizationDenied{}).EventType(),
	},
	"tenant": {
		(&spec.TenantCreated{}).EventType(),
		(&spec.TenantUpdated{}).EventType(),
		(&spec.TenantDisabled{}).EventType(),
		(&spec.TenantEnabled{}).EventType(),
		(&spec.TenantUserAttributeSchemaUpdated{}).EventType(),
	},
	"key": {
		(&spec.SigningKeyRotated{}).EventType(),
	},
}

func init() {
	authn := []string{}
	for _, k := range []string{"success", "fail", "aggregated"} {
		authn = append(authn, auditEventCategoryTypes[k]...)
	}
	auditEventCategoryTypes["authentication"] = authn
}

const adminAuditEventExportMaxLimit = 10000

func (d Deps) handleListAdminAuditEvents(c *echo.Context) error {
	actor, err := d.RequireAuditReader(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return core.NoStoreJSON(c, http.StatusOK, map[string]any{"events": []AdminAuditEventResponse{}})
	}
	query, err := parseAuditEventQuery(c, actor)
	if err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	records, err := d.AuditEventRepo.List(c.Request().Context(), query)
	if err != nil {
		return err
	}
	response := make([]AdminAuditEventResponse, len(records))
	for i, rec := range records {
		response[i] = toAdminAuditEventResponse(rec)
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"events": response})
}

func (d Deps) handleGetAdminAuditEvent(c *echo.Context) error {
	actor, err := d.RequireAuditReader(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	if d.AuditEventRepo == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	rec, err := d.AuditEventRepo.FindByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	if rec == nil {
		return core.WriteBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	if !auditEventVisibleTo(rec, actor) {
		// 別テナントのイベントは存在を隠す。
		return core.WriteBrowserError(c, http.StatusNotFound, "event_not_found", "監査イベントが存在しません")
	}
	return core.NoStoreJSON(c, http.StatusOK, toAdminAuditEventResponse(rec))
}

func (d Deps) handleExportAdminAuditEvents(c *echo.Context) error {
	actor, err := d.RequireAuditReader(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	query, err := parseAuditEventQuery(c, actor)
	if err != nil {
		return core.WriteBrowserError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	query.Limit = adminAuditEventExportMaxLimit
	var records []*oauthports.AuditEventRecord
	if d.AuditEventRepo != nil {
		records, err = d.AuditEventRepo.List(c.Request().Context(), query)
		if err != nil {
			return err
		}
	}
	response := make([]AdminAuditEventResponse, len(records))
	for i, rec := range records {
		response[i] = toAdminAuditEventResponse(rec)
	}
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"audit_events.json\"")
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"events": response})
}

// requireAuditReader は AdminAuditEventsRead パーミッションを満たすユーザーを返す。
// admin / system_admin のどちらでも通る。所属テナントの拘束は問わない (実際の
// テナント絞り込みは List のクエリ生成時に行う)。

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
	// category はイベントカテゴリ絞り込み (wi-44 統合: 認証サブ分類 + 管理操作カテゴリ)。
	if category := c.QueryParam("category"); category != "" {
		types, ok := auditEventCategoryTypes[category]
		if !ok {
			return oauthports.AuditEventQuery{}, errors.New("category が不正です")
		}
		q.Types = types
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

func toAdminAuditEventResponse(rec *oauthports.AuditEventRecord) AdminAuditEventResponse {
	return AdminAuditEventResponse{
		ID:         rec.ID,
		TenantID:   rec.TenantID,
		Type:       rec.Type,
		OccurredAt: rec.OccurredAt,
		Payload:    rec.Payload,
	}
}
