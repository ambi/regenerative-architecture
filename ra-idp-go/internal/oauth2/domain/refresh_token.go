// refresh token の domain。ADR-004 のローテーション + ファミリー失効の基盤。
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"ra-idp-go/internal/spec"
)

const (
	refreshTokenBytes       = 48
	refreshTokenTTL         = 14 * 24 * time.Hour
	refreshTokenAbsoluteTTL = 30 * 24 * time.Hour
)

type GeneratedRefreshToken struct {
	Token  string
	Record *spec.RefreshTokenRecord
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func GenerateInitialRefreshToken(clientID, sub string, scopes []string, sc *spec.SenderConstraint, now time.Time) (*GeneratedRefreshToken, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	familyID, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	rec := &spec.RefreshTokenRecord{
		ID:                id,
		Hash:              HashRefreshToken(token),
		FamilyID:          familyID,
		ClientID:          clientID,
		Sub:               sub,
		Scopes:            scopes,
		IssuedAt:          now,
		ExpiresAt:         now.Add(refreshTokenTTL),
		AbsoluteExpiresAt: now.Add(refreshTokenAbsoluteTTL),
		SenderConstraint:  sc,
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &GeneratedRefreshToken{Token: token, Record: rec}, nil
}

func RotateRefreshToken(parent *spec.RefreshTokenRecord, now time.Time) (*GeneratedRefreshToken, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	// expires_at は短い方を採用 (absolute_expires_at で打ち切り)。
	expires := now.Add(refreshTokenTTL)
	if expires.After(parent.AbsoluteExpiresAt) {
		expires = parent.AbsoluteExpiresAt
	}
	parentID := parent.ID
	rec := &spec.RefreshTokenRecord{
		ID:                id,
		Hash:              HashRefreshToken(token),
		FamilyID:          parent.FamilyID,
		ParentID:          &parentID,
		ClientID:          parent.ClientID,
		Sub:               parent.Sub,
		Scopes:            parent.Scopes,
		IssuedAt:          now,
		ExpiresAt:         expires,
		AbsoluteExpiresAt: parent.AbsoluteExpiresAt,
		SenderConstraint:  parent.SenderConstraint,
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	return &GeneratedRefreshToken{Token: token, Record: rec}, nil
}

func IsRefreshTokenReplay(rec *spec.RefreshTokenRecord) bool {
	return rec.Rotated || rec.Revoked
}

func IsRefreshTokenAbsoluteExpired(rec *spec.RefreshTokenRecord, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	return !now.Before(rec.AbsoluteExpiresAt)
}
