// Device Authorization Grant (RFC 8628) のドメインロジック。
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"ra-idp-go/internal/spec"
)

// user_code 文字集合 (0/O, 1/I/L の混同を避けた子音 + 数字回避)。RFC 8628 §6.1 推奨。
const (
	userCodeCharset = "BCDFGHJKLMNPQRSTVWXZ"
	userCodeLength  = 8
)

// DeviceCodeTTL は RFC 8628 §3.2 の expires_in。
const DeviceCodeTTL = 600 * time.Second

func GenerateDeviceCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func HashDeviceCode(deviceCode string) string {
	sum := sha256.Sum256([]byte(deviceCode))
	return hex.EncodeToString(sum[:])
}

func GenerateUserCode() (string, error) {
	var raw strings.Builder
	for range userCodeLength {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(userCodeCharset))))
		if err != nil {
			return "", err
		}
		raw.WriteByte(userCodeCharset[n.Int64()])
	}
	s := raw.String()
	return s[:4] + "-" + s[4:], nil
}

// NormalizeUserCode は索引キー化 (大文字化・記号除去)。
func NormalizeUserCode(input string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(input) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func IsDeviceExpired(rec *spec.DeviceAuthorization, now time.Time) bool {
	if rec.State == spec.DeviceFlowExpired {
		return true
	}
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(rec.ExpiresAt)
}
