// Package domain: OAuth2 ドメインモデル。
//
// PKCE (RFC 7636) verifier の検証。code_challenge_method=S256 のみサポート。
package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// VerifyPKCES256 は code_verifier の SHA-256 → base64url(no-padding) が
// code_challenge と一致するかを検証する。
func VerifyPKCES256(codeVerifier, codeChallenge string) bool {
	sum := sha256.Sum256([]byte(codeVerifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(codeChallenge)) == 1
}
