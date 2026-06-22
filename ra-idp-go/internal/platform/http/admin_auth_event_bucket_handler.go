package http

// SCL interface: ListAuthenticationEventBuckets (component: Authentication)。
// SCL permission: AdminAuditEventsRead を再利用する (集約も監査可視化の一部)。
// 攻撃時にログイン失敗を個別行へ落とさず集約した bucket を、所属テナント境界内で
// 新しい窓順に返す (wi-20 スライス 3)。書き込み経路は定義しない。

import (
	"net/http"
	"time"

	authusecases "ra-idp-go/internal/authentication/usecases"
	"ra-idp-go/internal/platform/http/core"

	"github.com/labstack/echo/v5"
)

type authEventBucketResponse struct {
	Kind        string    `json:"kind"`
	KeyHash     string    `json:"key_hash"`
	WindowStart time.Time `json:"window_start"`
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

func (d Deps) handleListAuthEventBuckets(c *echo.Context) error {
	actor, err := d.RequireAuditReader(c)
	if err != nil {
		return d.WriteAdminAccessError(c, err)
	}
	limit := parseLimitParam(c, authusecases.AuthEventBucketDefaultLimit)
	buckets, err := authusecases.ListAuthEventBuckets(
		c.Request().Context(), d.AuthEventBucketStore, actor.TenantID, limit,
	)
	if err != nil {
		return err
	}
	response := make([]authEventBucketResponse, len(buckets))
	for i, bucket := range buckets {
		response[i] = authEventBucketResponse{
			Kind:        bucket.Kind,
			KeyHash:     bucket.KeyHash,
			WindowStart: bucket.WindowStart,
			Count:       bucket.Count,
			FirstSeen:   bucket.FirstSeen,
			LastSeen:    bucket.LastSeen,
		}
	}
	return core.NoStoreJSON(c, http.StatusOK, map[string]any{"buckets": response})
}
