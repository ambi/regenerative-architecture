// Package bootstrap は ra-idp-go プロセスの起動・DI を司る。
// main.go はここを呼ぶだけで、エンドポイント追加・永続層差し替えは本パッケージ内で完結する。
package bootstrap

import (
	"os"
	"strings"
)

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
