// Package bootstrap は ra-idp-go プロセスの起動・DI を司る。
// main.go はここを呼ぶだけで、エンドポイント追加・永続層差し替えは本パッケージ内で完結する。
package bootstrap

import (
	"os"
	"strconv"
	"strings"
)

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
