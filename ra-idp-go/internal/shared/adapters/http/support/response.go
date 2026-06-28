package support

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"
)

// NoStoreJSON は Cache-Control: no-store を付けて JSON を返す。認証・認可に関わる
// レスポンスが中間キャッシュに残らないようにする共通ヘルパ。
func NoStoreJSON(c *echo.Context, status int, body any) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(status, body)
}

// WriteBrowserError はブラウザ向け API の {error, message} エラー body を返す。
func WriteBrowserError(c *echo.Context, status int, code, message string) error {
	return NoStoreJSON(c, status, map[string]string{"error": code, "message": message})
}

// DecodeJSON はリクエスト body を上限付き (64KiB) かつ未知フィールド拒否で復号する。
func DecodeJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
