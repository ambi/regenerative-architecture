package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

const PasswordResetTokenTTLSeconds = 1800

type RequestPasswordResetDeps struct {
	UserRepo    oauthports.UserRepository
	TokenStore  authnports.PasswordResetTokenStore
	EmailSender authnports.EmailSender
	Emit        func(spec.DomainEvent)
	Issuer      string
	TokenTTL    time.Duration
}

type RequestPasswordResetInput struct {
	Email string
	Now   time.Time
}

func RequestPasswordReset(ctx context.Context, deps RequestPasswordResetDeps, in RequestPasswordResetInput) error {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if deps.Emit != nil {
		deps.Emit(&spec.PasswordResetRequested{At: now, TenantID: tenancy.TenantID(ctx), EmailHash: sha256Hex(email)})
	}
	if email == "" {
		return nil
	}

	user, err := deps.UserRepo.FindByEmail(ctx, tenancy.TenantID(ctx), email)
	if err != nil {
		return err
	}
	if user == nil || !user.EmailVerified || user.Email == nil {
		return nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)
	ttl := deps.TokenTTL
	if ttl == 0 {
		ttl = PasswordResetTokenTTLSeconds * time.Second
	}
	if err := deps.TokenStore.Save(ctx, authnports.PasswordResetTokenRecord{
		Sub:       user.Sub,
		TokenHash: sha256Hex(rawToken),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}); err != nil {
		return err
	}

	resetURL := strings.TrimRight(deps.Issuer, "/") + "/reset_password?token=" + url.QueryEscape(rawToken)
	minutes := int(ttl.Round(time.Minute) / time.Minute)
	// Send to the verified address stored on the account, not the raw request
	// input, so untrusted request data never reaches the email content (CWE-640).
	delivered := deps.EmailSender.SendEmail(ctx, authnports.EmailMessage{
		To:      *user.Email,
		Subject: "Password reset",
		Text: fmt.Sprintf(
			"A password reset was requested for your account.\n\nOpen the link below within %d minutes to set a new password:\n%s\n\nIf you did not request this, you can safely ignore this email.",
			minutes, resetURL,
		),
	})
	if deps.Emit != nil {
		deps.Emit(&spec.EmailSent{
			At: now, ToHash: sha256Hex(email), Purpose: "password_reset", Delivered: delivered,
		})
	}
	return nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
