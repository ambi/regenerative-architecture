// /userinfo
package http

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/infrastructure/crypto"
	"ra-idp-go/internal/infrastructure/http/core"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

// effectiveUserAttributeDefs はテナントに有効な属性定義 (組み込み + tenant custom)
// を返す。AttrSchemaRepo 未設定時は組み込み定義のみ。
func (d Deps) effectiveUserAttributeDefs(ctx context.Context, tenantID string) ([]spec.UserAttributeDef, error) {
	defs := spec.BuiltinUserAttributeDefs()
	if d.AttrSchemaRepo == nil {
		return defs, nil
	}
	schema, err := d.AttrSchemaRepo.FindByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if schema != nil {
		defs = append(defs, schema.Attributes...)
	}
	return defs, nil
}

func (d Deps) handleUserInfo(c *echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	dpopHeader := c.Request().Header.Get("DPoP")
	bearer := strings.HasPrefix(auth, "Bearer ")
	dpopAuth := strings.HasPrefix(auth, "DPoP ")
	if !bearer && !dpopAuth {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "Bearer token が必要"))
	}
	var token string
	if bearer {
		token = strings.TrimPrefix(auth, "Bearer ")
	} else {
		token = strings.TrimPrefix(auth, "DPoP ")
	}
	intro, err := d.TokenIntrospector.IntrospectAccessToken(c.Request().Context(), token)
	if err != nil {
		return writeOAuthError(c, err)
	}
	if !intro.Active {
		return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "トークンが無効"))
	}
	if d.AccessTokenDenylist != nil && intro.JTI != "" {
		revoked, err := d.AccessTokenDenylist.IsRevoked(c.Request().Context(), intro.JTI)
		if err != nil {
			return writeOAuthError(c, err)
		}
		if revoked {
			return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "トークンが失効済みです"))
		}
	}
	if intro.SenderConstraint != nil {
		switch intro.SenderConstraint.Type {
		case spec.SenderConstraintMTLS:
			cert, err := crypto.ParseClientCertificateHeader(c.Request().Header.Get(clientCertHeader))
			if err != nil || subtle.ConstantTimeCompare(
				[]byte(cert.ThumbprintS256),
				[]byte(intro.SenderConstraint.X5TS256),
			) != 1 {
				return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "mTLS 証明書バインドが一致しません"))
			}
		case spec.SenderConstraintDPoP:
			if dpopHeader == "" || d.DpopReplayStore == nil {
				return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "DPoP proof が必要"))
			}
			r, err := crypto.VerifyDPoP(
				c.Request().Context(), dpopHeader,
				c.Request().Method, core.RequestHTU(c, d.Issuer),
				d.DpopReplayStore, time.Now().UTC(),
			)
			if err != nil || r == nil || subtle.ConstantTimeCompare(
				[]byte(r.JKT), []byte(intro.SenderConstraint.JKT),
			) != 1 {
				return writeOAuthError(c, usecases.NewOAuthError("invalid_token", "DPoP 鍵バインドが一致しません"))
			}
		}
	}
	res, err := usecases.UserInfo(c.Request().Context(), d.UserRepo, d.Authorizer, usecases.UserInfoInput{
		Scopes: strings.Fields(intro.Scope), Sub: intro.Sub, Active: intro.Active, ClientID: intro.ClientID,
		ResolveAttributeDefs: d.effectiveUserAttributeDefs,
	})
	if err != nil {
		return writeOAuthError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}
