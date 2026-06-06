// クライアント認証 (client_secret_basic / client_secret_post / private_key_jwt)
package http

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	oauthdomain "ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

type authedClient struct{ ID string }

func (d Deps) authenticateTokenClient(c *echo.Context) (authedClient, error) {
	basicAuth := c.Request().Header.Get("Authorization")
	hasBasic := strings.HasPrefix(basicAuth, "Basic ")
	hasSecret := c.Request().PostFormValue("client_secret") != ""
	hasAssertion := c.Request().PostFormValue("client_assertion") != "" ||
		c.Request().PostFormValue("client_assertion_type") != ""
	methods := 0
	for _, present := range []bool{hasBasic, hasSecret, hasAssertion} {
		if present {
			methods++
		}
	}
	if methods > 1 {
		return authedClient{}, usecases.NewOAuthError("invalid_request", "複数のクライアント認証方式が混在しています")
	}

	// 1. client_assertion (private_key_jwt)
	if hasAssertion {
		const assertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
		if c.Request().PostFormValue("client_assertion_type") != assertionType {
			return authedClient{}, usecases.NewOAuthError("invalid_request", "未対応の client_assertion_type です")
		}
		a := c.Request().PostFormValue("client_assertion")
		if a == "" {
			return authedClient{}, usecases.NewOAuthError("invalid_request", "client_assertion が必要です")
		}
		clientID := c.Request().PostFormValue("client_id")
		client, err := d.ClientRepo.FindByID(c.Request().Context(), clientID)
		if err != nil || client == nil || client.TokenEndpointAuthMethod != spec.AuthMethodPrivateKeyJwt {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "クライアント認証に失敗しました")
		}
		resolver := d.JWKResolver
		if resolver == nil {
			resolver = crypto.NewJWKResolver()
		}
		_, err = crypto.VerifyClientAssertion(
			c.Request().Context(), a, clientID,
			acceptableClientAssertionAudiences(d.Issuer, c.Request()),
			func(ctx context.Context, cid string) ([]map[string]any, error) {
				cl, err := d.ClientRepo.FindByID(ctx, cid)
				if err != nil {
					return nil, err
				}
				if cl == nil {
					return nil, usecases.NewOAuthError("invalid_client", "client not found")
				}
				return resolver.Resolve(ctx, cl)
			},
			d.ClientAssertionReplayStore, time.Now().UTC(), nil,
		)
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", err.Error())
		}
		return authedClient{ID: clientID}, nil
	}

	// 2. client_secret_basic / client_secret_post
	var clientID, secret string
	method := spec.AuthMethodNone
	switch {
	case hasBasic:
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(basicAuth, "Basic "))
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "Basic 復号失敗")
		}
		parts := strings.SplitN(string(raw), ":", 2)
		if len(parts) != 2 {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "Basic 形式不正")
		}
		clientID, err = url.QueryUnescape(parts[0])
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "client_id の形式不正")
		}
		secret, err = url.QueryUnescape(parts[1])
		if err != nil {
			return authedClient{}, usecases.NewOAuthError("invalid_client", "client_secret の形式不正")
		}
		method = spec.AuthMethodClientSecretBasic
	case hasSecret:
		clientID = c.Request().PostFormValue("client_id")
		secret = c.Request().PostFormValue("client_secret")
		method = spec.AuthMethodClientSecretPost
	default:
		clientID = c.Request().PostFormValue("client_id")
	}
	client, err := d.ClientRepo.FindByID(c.Request().Context(), clientID)
	if err != nil || client == nil {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "未知の client_id")
	}
	if client.TokenEndpointAuthMethod != method {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "宣言されたクライアント認証方式と一致しません")
	}
	if method == spec.AuthMethodNone {
		return authedClient{ID: clientID}, nil
	}
	if client.ClientSecretHash == nil || !oauthdomain.VerifyClientSecret(secret, *client.ClientSecretHash) {
		return authedClient{}, usecases.NewOAuthError("invalid_client", "client_secret 不一致")
	}
	return authedClient{ID: clientID}, nil
}

func acceptableClientAssertionAudiences(issuer string, req *http.Request) []string {
	base := strings.TrimSuffix(issuer, "/")
	values := []string{base, base + "/token", base + "/par", base + "/introspect", base + "/revoke"}
	if req != nil {
		values = append(values, base+req.URL.Path)
	}
	return values
}
