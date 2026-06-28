// /health: 起動構成をそのまま返す簡易ヘルスエンドポイント。
package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (d Deps) handleHealth(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":        "ok",
		"persistence":   d.HealthInfo.Persistence,
		"event_sink":    d.HealthInfo.EventSink,
		"observability": d.HealthInfo.Observability,
		"authzen":       d.HealthInfo.AuthZEN,
	})
}
