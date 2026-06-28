// 認可コードのドメインモデル。RFC 9700 §4.10 の単一使用 + TTL。
package domain

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"ra-idp-go/internal/shared/spec"
)

const authCodeBytes = 32

type AuthorizationCodeInput struct {
	TenantID               string
	AuthorizationRequestID string
	ClientID               string
	Sub                    string
	Scopes                 []string
	RedirectURI            string
	CodeChallenge          string
	CodeChallengeMethod    spec.CodeChallengeMethod
	Nonce                  *string
	AuthTime               int64
	TTLSeconds             int
	Now                    time.Time
}

func GenerateAuthorizationCode(in AuthorizationCodeInput) (*spec.AuthorizationCodeRecord, error) {
	if in.TenantID == "" {
		in.TenantID = spec.DefaultTenantID
	}
	if in.TTLSeconds == 0 {
		in.TTLSeconds = 60
	}
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	b := make([]byte, authCodeBytes)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	rec := &spec.AuthorizationCodeRecord{
		Code:                   base64.RawURLEncoding.EncodeToString(b),
		TenantID:               in.TenantID,
		AuthorizationRequestID: in.AuthorizationRequestID,
		ClientID:               in.ClientID,
		Sub:                    in.Sub,
		Scopes:                 in.Scopes,
		RedirectURI:            in.RedirectURI,
		CodeChallenge:          in.CodeChallenge,
		CodeChallengeMethod:    in.CodeChallengeMethod,
		Nonce:                  in.Nonce,
		AuthTime:               in.AuthTime,
		State:                  spec.AuthCodeRecordIssued,
		IssuedAt:               in.Now,
		ExpiresAt:              in.Now.Add(time.Duration(in.TTLSeconds) * time.Second),
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return rec, nil
}

func IsCodeExpired(rec *spec.AuthorizationCodeRecord, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(rec.ExpiresAt)
}

func IsCodeRedeemed(rec *spec.AuthorizationCodeRecord) bool {
	return rec.RedeemedAt != nil
}
