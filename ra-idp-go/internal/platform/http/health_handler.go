// /health: 起動構成をそのまま返す簡易ヘルスエンドポイント。
package http

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// HealthInfo は bootstrap が決定した実行時構成のラベル。
// /health がそのまま JSON で返すだけの読み取り専用情報を保持する。
type HealthInfo struct {
	Persistence   string
	EventSink     string
	Observability string
	AuthZEN       string
}

func (d Deps) handleHealth(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":        "ok",
		"persistence":   d.HealthInfo.Persistence,
		"event_sink":    d.HealthInfo.EventSink,
		"observability": d.HealthInfo.Observability,
		"authzen":       d.HealthInfo.AuthZEN,
	})
}
