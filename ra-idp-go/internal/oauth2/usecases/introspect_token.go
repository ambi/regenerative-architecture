// /introspect (RFC 7662) の中核。access_token (JWT) と refresh_token (ストア) の両方を処理。
package usecases

import (
	"context"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

type IntrospectInput struct {
	Token         string
	TokenTypeHint string // "access_token" | "refresh_token" | ""
}

type IntrospectionResponse struct {
	Active    bool              `json:"active"`
	Scope     string            `json:"scope,omitempty"`
	ClientID  string            `json:"client_id,omitempty"`
	Sub       string            `json:"sub,omitempty"`
	Aud       []string          `json:"aud,omitempty"`
	TokenType string            `json:"token_type,omitempty"`
	Exp       int64             `json:"exp,omitempty"`
	Iat       int64             `json:"iat,omitempty"`
	JTI       string            `json:"jti,omitempty"`
	CNF       map[string]string `json:"cnf,omitempty"`
	Act       map[string]any    `json:"act,omitempty"`
	// AuthorizationDetails は RFC 9396 — RS が信頼する検証点 (ADR-050)。
	AuthorizationDetails []spec.AuthorizationDetail `json:"authorization_details,omitempty"`
}

type IntrospectDeps struct {
	Introspector        ports.TokenIntrospector
	RefreshStore        ports.RefreshTokenStore
	AccessTokenDenylist ports.AccessTokenDenylist
}

func IntrospectToken(ctx context.Context, deps IntrospectDeps, in IntrospectInput, now time.Time) (*IntrospectionResponse, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	// refresh_token として先に試す（hint=refresh_token か空）
	if in.TokenTypeHint == "" || in.TokenTypeHint == "refresh_token" {
		hash := domain.HashRefreshToken(in.Token)
		rec, err := deps.RefreshStore.FindByHash(ctx, hash)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			if rec.TenantID != tenancy.TenantID(ctx) {
				return &IntrospectionResponse{Active: false}, nil
			}
			active := !rec.Revoked && !rec.Rotated && now.Before(rec.AbsoluteExpiresAt)
			if !active {
				return &IntrospectionResponse{Active: false}, nil
			}
			resp := &IntrospectionResponse{
				Active:    true,
				Scope:     strings.Join(rec.Scopes, " "),
				ClientID:  rec.ClientID,
				Sub:       rec.Sub,
				TokenType: "refresh_token",
				Iat:       rec.IssuedAt.Unix(),
				Exp:       rec.ExpiresAt.Unix(),
				JTI:       rec.ID,
			}
			if rec.SenderConstraint != nil {
				resp.CNF = map[string]string{}
				if rec.SenderConstraint.JKT != "" {
					resp.CNF["jkt"] = rec.SenderConstraint.JKT
				}
				if rec.SenderConstraint.X5TS256 != "" {
					resp.CNF["x5t#S256"] = rec.SenderConstraint.X5TS256
				}
			}
			return resp, nil
		}
	}
	// access_token として検証
	r, err := deps.Introspector.IntrospectAccessToken(ctx, in.Token)
	if err != nil {
		return nil, err
	}
	if r.Active && r.JTI != "" && deps.AccessTokenDenylist != nil {
		revoked, err := deps.AccessTokenDenylist.IsRevoked(ctx, r.JTI)
		if err != nil {
			return nil, err
		}
		if revoked {
			return &IntrospectionResponse{Active: false}, nil
		}
	}
	resp := &IntrospectionResponse{
		Active:               r.Active,
		Scope:                r.Scope,
		ClientID:             r.ClientID,
		Sub:                  r.Sub,
		Aud:                  r.Aud,
		TokenType:            r.TokenType,
		Exp:                  r.Exp,
		Iat:                  r.Iat,
		JTI:                  r.JTI,
		Act:                  r.Act,
		AuthorizationDetails: r.AuthorizationDetails,
	}
	if r.SenderConstraint != nil {
		resp.CNF = map[string]string{}
		if r.SenderConstraint.JKT != "" {
			resp.CNF["jkt"] = r.SenderConstraint.JKT
		}
		if r.SenderConstraint.X5TS256 != "" {
			resp.CNF["x5t#S256"] = r.SenderConstraint.X5TS256
		}
	}
	return resp, nil
}
