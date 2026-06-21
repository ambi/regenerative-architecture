package http

import (
	stdcrypto "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/oauth2/domain"
	"ra-idp-go/internal/platform/persistence/memory"
	"ra-idp-go/internal/spec"

	"github.com/labstack/echo/v5"
)

func clientAuthServer(method spec.TokenEndpointAuthMethod) *echo.Echo {
	repo := memory.NewClientRepository()
	var secretHash *string
	if method == spec.AuthMethodClientSecretBasic || method == spec.AuthMethodClientSecretPost {
		hash := domain.HashClientSecret("secret")
		secretHash = &hash
	}
	clientType := spec.ClientConfidential
	if method == spec.AuthMethodNone {
		clientType = spec.ClientPublic
	}
	var subjectDN *string
	if method == spec.AuthMethodTlsClientAuth {
		value := "CN=client"
		subjectDN = &value
	}
	repo.Seed(&spec.Client{
		ClientID: "client", ClientSecretHash: secretHash, ClientType: clientType,
		RedirectURIs: []string{"https://client.example/cb"},
		GrantTypes:   []spec.GrantType{spec.GrantClientCredentials},
		ResponseTypes: []spec.ResponseType{
			spec.ResponseTypeCode,
		},
		TokenEndpointAuthMethod:  method,
		TlsClientAuthSubjectDN:   subjectDN,
		Scope:                    "api",
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                time.Now(),
	})
	deps := Deps{Issuer: "https://idp.example", ClientRepo: repo}
	e := echo.New()
	e.POST("/test", func(c *echo.Context) error {
		if err := c.Request().ParseForm(); err != nil {
			return err
		}
		client, err := deps.authenticateTokenClient(c)
		if err != nil {
			return writeOAuthError(c, err)
		}
		return c.String(http.StatusOK, client.ID)
	})
	return e
}

func TestTLSClientAuthentication(t *testing.T) {
	header := clientCertificateHeader(t, "client")
	e := clientAuthServer(spec.AuthMethodTlsClientAuth)
	form := url.Values{"client_id": {"client"}}
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(clientCertHeader, header)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func clientCertificateHeader(t *testing.T, commonName string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return url.QueryEscape(string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
}

func TestClientAuthenticationMethods(t *testing.T) {
	t.Run("basic succeeds only when declared", func(t *testing.T) {
		e := clientAuthServer(spec.AuthMethodClientSecretBasic)
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(url.Values{}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("client", "secret")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}

		e = clientAuthServer(spec.AuthMethodClientSecretPost)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("basic authentication accepted for client_secret_post client")
		}
	})

	t.Run("post and none succeed when declared", func(t *testing.T) {
		for _, tc := range []struct {
			method spec.TokenEndpointAuthMethod
			form   url.Values
		}{
			{spec.AuthMethodClientSecretPost, url.Values{"client_id": {"client"}, "client_secret": {"secret"}}},
			{spec.AuthMethodNone, url.Values{"client_id": {"client"}}},
		} {
			e := clientAuthServer(tc.method)
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status=%d body=%s", tc.method, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("mixed methods are rejected", func(t *testing.T) {
		e := clientAuthServer(spec.AuthMethodClientSecretBasic)
		form := url.Values{"client_id": {"client"}, "client_secret": {"secret"}}
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("client", "secret")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatal("mixed authentication methods were accepted")
		}
	})
}

func TestClientAuthenticationFailuresAreUniform(t *testing.T) {
	e := clientAuthServer(spec.AuthMethodClientSecretBasic)
	request := func(clientID, secret string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
		req.SetBasicAuth(clientID, secret)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	responses := []*httptest.ResponseRecorder{
		request("unknown", "secret"),
		request("client", "wrong-secret"),
	}
	for _, response := range responses {
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if body := response.Body.String(); body != `{"error":"invalid_client","error_description":"クライアント認証に失敗しました"}`+"\n" {
			t.Fatalf("unexpected body: %s", body)
		}
	}

	postServer := clientAuthServer(spec.AuthMethodClientSecretPost)
	req := httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	req.SetBasicAuth("client", "secret")
	rec := httptest.NewRecorder()
	postServer.ServeHTTP(rec, req)
	if rec.Body.String() != responses[0].Body.String() {
		t.Fatalf("auth method mismatch body=%s, want %s", rec.Body.String(), responses[0].Body.String())
	}
}

func TestPrivateKeyJWTAuthentication(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := map[string]any{
		"kty": "RSA",
		"kid": "key-1",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
	repo := memory.NewClientRepository()
	repo.Seed(&spec.Client{
		ClientID: "private-client", ClientType: spec.ClientConfidential,
		RedirectURIs: []string{"https://client.example/cb"},
		GrantTypes:   []spec.GrantType{spec.GrantClientCredentials},
		ResponseTypes: []spec.ResponseType{
			spec.ResponseTypeCode,
		},
		TokenEndpointAuthMethod:  spec.AuthMethodPrivateKeyJwt,
		Scope:                    "api",
		JWKS:                     map[string]any{"keys": []any{jwk}},
		IDTokenSignedResponseAlg: spec.SigAlgPS256,
		FapiProfile:              spec.FapiNone,
		CreatedAt:                time.Now(),
	})
	deps := Deps{
		Issuer: "https://idp.example", ClientRepo: repo,
		ClientAssertionReplayStore: memory.NewClientAssertionReplayStore(),
	}
	e := echo.New()
	e.POST("/test", func(c *echo.Context) error {
		if err := c.Request().ParseForm(); err != nil {
			return err
		}
		client, err := deps.authenticateTokenClient(c)
		if err != nil {
			return writeOAuthError(c, err)
		}
		return c.String(http.StatusOK, client.ID)
	})

	now := time.Now()
	header, _ := json.Marshal(map[string]any{"alg": "PS256", "kid": "key-1"})
	payload, _ := json.Marshal(map[string]any{
		"iss": "private-client", "sub": "private-client",
		"aud": "https://idp.example/test", "jti": "jti-private",
		"iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	})
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPSS(
		rand.Reader,
		key,
		stdcrypto.SHA256,
		digest[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		t.Fatal(err)
	}
	assertion := signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	form := url.Values{
		"client_id":             {"private-client"},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
		"client_assertion":      {assertion},
	}
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
