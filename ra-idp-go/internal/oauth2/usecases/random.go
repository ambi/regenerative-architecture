package usecases

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// generateOpaqueToken は base64url(no-padding) で n バイトのランダム値を返す。
// authorization_code 等の opaque token に使用する。
func generateOpaqueToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
