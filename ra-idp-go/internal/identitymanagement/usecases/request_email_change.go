package usecases

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	authnports "ra-idp-go/internal/authentication/ports"
	oauthports "ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

const EmailChangeTokenTTLSeconds = 1800

var (
	ErrInvalidEmail   = errors.New("email is not a valid address")
	ErrEmailUnchanged = errors.New("email is unchanged")
	ErrEmailTaken     = errors.New("email is already in use")
)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// RequestEmailChangeDeps / Input は primary email 変更の起票 (self-service, wi-21)。
// 新アドレスへワンタイムリンクを送り、確定は ConfirmEmailChange で行う。実際の
// User.Email 更新は確定時まで起きない (新アドレスの所有確認を経るまで反映しない)。
type RequestEmailChangeDeps struct {
	UserRepo    oauthports.UserRepository
	TokenStore  authnports.EmailChangeTokenStore
	EmailSender authnports.EmailSender
	Emit        func(spec.DomainEvent)
	Issuer      string
	TokenTTL    time.Duration
}

type RequestEmailChangeInput struct {
	Sub      string
	NewEmail string
	Now      time.Time
}

func RequestEmailChange(ctx context.Context, deps RequestEmailChangeDeps, in RequestEmailChangeInput) error {
	now := in.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	addr, err := mail.ParseAddress(strings.TrimSpace(in.NewEmail))
	if err != nil {
		return ErrInvalidEmail
	}
	newEmail := strings.ToLower(addr.Address)

	user, err := deps.UserRepo.FindBySub(ctx, in.Sub)
	if err != nil {
		return err
	}
	if user == nil || user.TenantID != tenancy.TenantID(ctx) {
		return ErrUserNotFound
	}
	if user.Email != nil && user.EmailVerified && strings.EqualFold(*user.Email, newEmail) {
		return ErrEmailUnchanged
	}
	// tenant 内で他ユーザが使っているアドレスは拒否する。
	existing, err := deps.UserRepo.FindByEmail(ctx, user.TenantID, newEmail)
	if err != nil {
		return err
	}
	if existing != nil && existing.Sub != user.Sub {
		return ErrEmailTaken
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)
	ttl := deps.TokenTTL
	if ttl == 0 {
		ttl = EmailChangeTokenTTLSeconds * time.Second
	}
	if err := deps.TokenStore.Save(ctx, authnports.EmailChangeTokenRecord{
		Sub: user.Sub, TokenHash: sha256Hex(rawToken), NewEmail: newEmail,
		CreatedAt: now, ExpiresAt: now.Add(ttl),
	}); err != nil {
		return err
	}

	verifyURL := strings.TrimRight(deps.Issuer, "/") + "/account/email/verify?token=" + url.QueryEscape(rawToken)
	minutes := int(ttl.Round(time.Minute) / time.Minute)
	delivered := deps.EmailSender.SendEmail(ctx, authnports.EmailMessage{
		To:      newEmail,
		Subject: "Confirm your new email address",
		Text: fmt.Sprintf(
			"A request was made to set this address as the email for your account.\n\nOpen the link below within %d minutes to confirm:\n%s\n\nIf you did not request this, you can ignore this email.",
			minutes, verifyURL,
		),
	})
	if deps.Emit != nil {
		deps.Emit(&spec.EmailChangeRequested{
			At: now, TenantID: user.TenantID, Sub: user.Sub, NewEmailHash: sha256Hex(newEmail),
		})
		deps.Emit(&spec.EmailSent{
			At: now, ToHash: sha256Hex(newEmail), Purpose: "email_change", Delivered: delivered,
		})
	}
	return nil
}
