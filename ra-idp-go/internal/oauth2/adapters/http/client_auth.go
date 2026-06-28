// クライアント認証 (client_secret_basic / client_secret_post / private_key_jwt)
package http

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	oauthdomain "ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/shared/adapters/crypto"
	"ra-idp-go/internal/shared/adapters/http/support"
	"ra-idp-go/internal/shared/spec"

	"github.com/labstack/echo/v5"
)

const clientCertHeader = "X-Client-Certificate"

const invalidClientDescription = "クライアント認証に失敗しました"

var dummyClientSecretHash = oauthdomain.HashClientSecret("ra-idp-invalid-client")

type authedClient struct {
	ID                 string
	MTLSThumbprintS256 string
}

func (d Deps) authenticateTokenClient(c *echo.Context) (authedClient, error) {
	basicAuth := c.Request().Header.Get("Authorization")
	hasBasic := strings.HasPrefix(basicAuth, "Basic ")
	hasSecret := c.Request().PostFormValue("client_secret") != ""
	hasAssertion := c.Request().PostFormValue("client_assertion") != "" ||
		c.Request().PostFormValue("client_assertion_type") != ""
	hasCertificate := c.Request().Header.Get(clientCertHeader) != ""
	methods := 0
	for _, present := range []bool{hasBasic, hasSecret, hasAssertion, hasCertificate} {
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
		client, err := d.ClientRepo.FindByID(c.Request().Context(), support.RequestTenantID(c), clientID)
		if err != nil || client == nil || client.TokenEndpointAuthMethod != spec.AuthMethodPrivateKeyJwt {
			return authedClient{}, invalidClientError()
		}
		resolver := d.JWKResolver
		if resolver == nil {
			resolver = crypto.NewJWKResolver()
		}
		_, err = crypto.VerifyClientAssertion(
			c.Request().Context(), a, clientID,
			acceptableClientAssertionAudiences(support.RequestIssuer(c, d.Issuer), c.Request()),
			func(ctx context.Context, cid string) ([]map[string]any, error) {
				cl, err := d.ClientRepo.FindByID(ctx, support.RequestTenantID(c), cid)
				if err != nil {
					return nil, err
				}
				if cl == nil {
					return nil, invalidClientError()
				}
				return resolver.Resolve(ctx, cl)
			},
			d.ClientAssertionReplayStore, time.Now().UTC(), nil,
		)
		if err != nil {
			return authedClient{}, invalidClientError()
		}
		return authedClient{ID: clientID}, nil
	}

	// 2. tls_client_auth
	if hasCertificate {
		clientID := c.Request().PostFormValue("client_id")
		client, err := d.ClientRepo.FindByID(c.Request().Context(), support.RequestTenantID(c), clientID)
		if err != nil || client == nil ||
			client.TokenEndpointAuthMethod != spec.AuthMethodTlsClientAuth ||
			client.TlsClientAuthSubjectDN == nil {
			return authedClient{}, invalidClientError()
		}
		cert, err := crypto.ParseClientCertificateHeader(c.Request().Header.Get(clientCertHeader))
		if err != nil || !crypto.ClientCertSubjectMatches(*client.TlsClientAuthSubjectDN, cert.SubjectDN) {
			return authedClient{}, invalidClientError()
		}
		return authedClient{ID: clientID, MTLSThumbprintS256: cert.ThumbprintS256}, nil
	}

	// 3. client_secret_basic / client_secret_post
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
	client, err := d.ClientRepo.FindByID(c.Request().Context(), support.RequestTenantID(c), clientID)
	if err != nil || client == nil {
		if method != spec.AuthMethodNone {
			oauthdomain.VerifyClientSecret(secret, dummyClientSecretHash)
		}
		return authedClient{}, invalidClientError()
	}
	if client.TokenEndpointAuthMethod != method {
		if method != spec.AuthMethodNone {
			hash := dummyClientSecretHash
			if client.ClientSecretHash != nil {
				hash = *client.ClientSecretHash
			}
			oauthdomain.VerifyClientSecret(secret, hash)
		}
		return authedClient{}, invalidClientError()
	}
	if method == spec.AuthMethodNone {
		return authedClient{ID: clientID}, nil
	}
	if client.ClientSecretHash == nil || !oauthdomain.VerifyClientSecret(secret, *client.ClientSecretHash) {
		return authedClient{}, invalidClientError()
	}
	return authedClient{ID: clientID}, nil
}

func invalidClientError() error {
	return usecases.NewOAuthError("invalid_client", invalidClientDescription)
}

func acceptableClientAssertionAudiences(issuer string, req *http.Request) []string {
	base := strings.TrimSuffix(issuer, "/")
	values := []string{base, base + "/token", base + "/par", base + "/introspect", base + "/revoke"}
	if req != nil {
		values = append(values, base+req.URL.Path)
	}
	return values
}
