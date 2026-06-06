package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

func HashClientSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func VerifyClientSecret(secret, encodedHash string) bool {
	actual := HashClientSecret(secret)
	if len(actual) != len(encodedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(encodedHash)) == 1
}
