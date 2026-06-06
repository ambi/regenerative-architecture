// Session manager: login session の生成・解決・破棄。
// Cookie 名と TTL を 1 箇所に集約する。
package usecases

import (
	"context"
	"net/url"
	"strings"
	"time"

	"ra-idp-go/internal/authentication/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
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
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id, err := spec.NewUUIDv4()
	if err != nil {
		return nil, err
	}
	sess := &spec.LoginSession{
		ID:        id,
		Sub:       sub,
		AuthTime:  now.Unix(),
		ExpiresAt: now.Add(SessionTTLSeconds * time.Second),
	}
	if err := m.Store.Save(ctx, sess); err != nil {
		return nil, err
	}
	return &domain.AuthenticationContext{
		Sub:       sub,
		AuthTime:  sess.AuthTime,
		AMR:       amr,
		SessionID: id,
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
	return &domain.AuthenticationContext{
		Sub:       sess.Sub,
		AuthTime:  sess.AuthTime,
		AMR:       []string{"pwd"},
		SessionID: sess.ID,
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
