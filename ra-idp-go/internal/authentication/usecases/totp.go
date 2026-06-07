package usecases

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 authenticator compatibility requires HMAC-SHA1.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
)

const (
	TOTPStepSeconds = int64(30)
	TOTPDigits      = 6
	TOTPWindow      = 1
	TOTPSecretBytes = 20

	totpStepSeconds = TOTPStepSeconds
	totpDigits      = TOTPDigits
	totpSecretBytes = TOTPSecretBytes
)

func GenerateTOTPSecret() (string, error) {
	secret := make([]byte, totpSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

func GenerateTOTP(secretBase32 string, unixSeconds int64) (string, error) {
	if unixSeconds < 0 {
		return "", fmt.Errorf("TOTP time must not be negative")
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretBase32)
	if err != nil {
		return "", fmt.Errorf("decode TOTP secret: %w", err)
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(unixSeconds/totpStepSeconds))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000), nil
}

func VerifyTOTP(secretBase32, submitted string, unixSeconds int64, window int) bool {
	if len(submitted) != totpDigits {
		return false
	}
	for _, r := range submitted {
		if r < '0' || r > '9' {
			return false
		}
	}
	for i := -window; i <= window; i++ {
		candidate, err := GenerateTOTP(secretBase32, unixSeconds+int64(i)*totpStepSeconds)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(submitted)) == 1 {
			return true
		}
	}
	return false
}

func BuildOTPAuthURI(secretBase32, accountName, issuer string) string {
	query := url.Values{
		"secret":    {secretBase32},
		"issuer":    {issuer},
		"algorithm": {"SHA1"},
		"digits":    {"6"},
		"period":    {"30"},
	}
	return "otpauth://totp/" + url.PathEscape(issuer+":"+accountName) + "?" + query.Encode()
}
