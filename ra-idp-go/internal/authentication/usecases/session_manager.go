// Session manager: login session の生成・解決・破棄。
// Cookie 名と TTL を 1 箇所に集約する。
package usecases

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/shared/spec"
	"ra-idp-go/internal/tenancy"
)

const (
	SessionCookie     = "ra_idp_session"
	SessionTTLSeconds = 3600
)

type SessionManager struct {
	Store ports.SessionStore
}

func NewSessionManager(s ports.SessionStore) *SessionManager {
	return &SessionManager{Store: s}
}

func (m *SessionManager) Create(ctx context.Context, sub string, amr []string, now time.Time) (*domain.AuthenticationContext, error) {
	return m.CreateWithPending(ctx, sub, amr, now, false)
}

func (m *SessionManager) CreateWithPending(
	ctx context.Context,
	sub string,
	amr []string,
	now time.Time,
	authenticationPending bool,
) (*domain.AuthenticationContext, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	sess := &spec.LoginSession{
		ID:                    id,
		TenantID:              tenancy.TenantID(ctx),
		Sub:                   sub,
		AuthTime:              now.Unix(),
		AMR:                   amr,
		ACR:                   DeriveACR(amr),
		AuthenticationPending: authenticationPending,
		ExpiresAt:             now.Add(SessionTTLSeconds * time.Second),
	}
	if err := m.Store.Save(ctx, sess); err != nil {
		return nil, err
	}
	return &domain.AuthenticationContext{
		Sub:                   sub,
		AuthTime:              sess.AuthTime,
		AMR:                   amr,
		ACR:                   sess.ACR,
		SessionID:             id,
		AuthenticationPending: sess.AuthenticationPending,
	}, nil
}

func (m *SessionManager) CompleteFactor(
	ctx context.Context,
	sessionID string,
	additionalAMR []string,
) (*domain.AuthenticationContext, error) {
	sess, err := m.Store.Find(ctx, sessionID)
	if err != nil || sess == nil {
		return nil, err
	}
	if sess.TenantID != tenancy.TenantID(ctx) {
		return nil, nil
	}
	merged := slices.Clone(sess.AMR)
	for _, method := range additionalAMR {
		if !slices.Contains(merged, method) {
			merged = append(merged, method)
		}
	}
	sess.AMR = merged
	sess.ACR = DeriveACR(merged)
	sess.AuthenticationPending = false
	if err := m.Store.Save(ctx, sess); err != nil {
		return nil, err
	}
	return &domain.AuthenticationContext{
		Sub:                   sess.Sub,
		AuthTime:              sess.AuthTime,
		AMR:                   slices.Clone(sess.AMR),
		ACR:                   sess.ACR,
		SessionID:             sess.ID,
		AuthenticationPending: sess.AuthenticationPending,
		StepUpAt:              sess.StepUpAt,
	}, nil
}

// RecordStepUp は session に step-up 再認証の成立時刻を刻む (ADR-043)。pending な
// session や別テナントの session には作用させない。成立後の AuthenticationContext を返す。
func (m *SessionManager) RecordStepUp(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (*domain.AuthenticationContext, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sess, err := m.Store.Find(ctx, sessionID)
	if err != nil || sess == nil {
		return nil, err
	}
	if sess.TenantID != tenancy.TenantID(ctx) || sess.AuthenticationPending {
		return nil, nil
	}
	sess.StepUpAt = now.Unix()
	if err := m.Store.Save(ctx, sess); err != nil {
		return nil, err
	}
	return &domain.AuthenticationContext{
		Sub:                   sess.Sub,
		AuthTime:              sess.AuthTime,
		AMR:                   slices.Clone(sess.AMR),
		ACR:                   sess.ACR,
		SessionID:             sess.ID,
		AuthenticationPending: sess.AuthenticationPending,
		StepUpAt:              sess.StepUpAt,
	}, nil
}

func (m *SessionManager) Resolve(ctx context.Context, headers domain.Headers) (*domain.AuthenticationContext, error) {
	sid := parseCookies(headers.Get("Cookie"))[SessionCookie]
	if sid == "" {
		return nil, nil
	}
	sess, err := m.Store.Find(ctx, sid)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, nil
	}
	if sess.TenantID != tenancy.TenantID(ctx) {
		return nil, nil
	}
	return &domain.AuthenticationContext{
		Sub:                   sess.Sub,
		AuthTime:              sess.AuthTime,
		AMR:                   sess.AMR,
		ACR:                   sess.ACR,
		SessionID:             sess.ID,
		AuthenticationPending: sess.AuthenticationPending,
		StepUpAt:              sess.StepUpAt,
	}, nil
}

func (m *SessionManager) Revoke(ctx context.Context, cookieHeader string) error {
	sid := parseCookies(cookieHeader)[SessionCookie]
	if sid == "" {
		return nil
	}
	return m.Store.Delete(ctx, sid)
}

func parseCookies(header string) map[string]string {
	out := map[string]string{}
	if header == "" {
		return out
	}
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		name, value, ok := strings.Cut(part, "=")
		if !ok || name == "" {
			continue
		}
		if dec, err := url.QueryUnescape(value); err == nil {
			out[name] = dec
		} else {
			out[name] = value
		}
	}
	return out
}
