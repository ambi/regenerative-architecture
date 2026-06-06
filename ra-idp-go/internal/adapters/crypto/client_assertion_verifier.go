// Package crypto: private_key_jwt クライアント認証 (RFC 7523) 検証。
package crypto

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/ports"
)

const (
	clientAssertionMaxLifetimeSeconds = 300
	clientAssertionClockSkewSeconds   = 60
)

type ClientAssertionResult struct {
	ClientID string
}

// VerifyClientAssertion は client_assertion JWT を検証する。
//   - クライアントの jwks_uri から取得した JWKS (本実装は事前注入された keysFn) で署名検証
//   - aud がトークンエンドポイント URL に一致
//   - iss == sub == client_id
//   - exp / nbf チェック
//   - jti をリプレイストアに登録
func VerifyClientAssertion(
	ctx context.Context,
	assertion, expectedClientID string,
	expectedAudiences []string,
	keysFn func(ctx context.Context, clientID string) ([]map[string]any, error),
	replay ports.ClientAssertionReplayStore,
	now time.Time,
	httpc *http.Client,
) (*ClientAssertionResult, error) {
	_ = httpc // 本実装では keysFn 経由で取得する。HTTP 実装は keysFn 側で差し込む。

	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		return nil, errors.New("client_assertion: malformed JWT")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var header map[string]any
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, err
	}
	alg, _ := header["alg"].(string)
	if alg != "PS256" && alg != "ES256" {
		return nil, fmt.Errorf("client_assertion: alg %q not allowed", alg)
	}
	kid, _ := header["kid"].(string)

	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(pb, &payload); err != nil {
		return nil, err
	}
	iss, _ := payload["iss"].(string)
	sub, _ := payload["sub"].(string)
	if iss == "" || iss != sub {
		return nil, errors.New("client_assertion: iss must equal sub")
	}
	if iss != expectedClientID {
		return nil, errors.New("client_assertion: iss does not match client_id")
	}
	if !verifyAudience(payload["aud"], expectedAudiences) {
		return nil, errors.New("client_assertion: aud mismatch")
	}
	exp, ok := payload["exp"].(float64)
	if !ok {
		return nil, errors.New("client_assertion: exp required")
	}
	if int64(exp)+clientAssertionClockSkewSeconds < now.Unix() {
		return nil, errors.New("client_assertion: expired")
	}
	if int64(exp)-now.Unix() > clientAssertionMaxLifetimeSeconds+clientAssertionClockSkewSeconds {
		return nil, errors.New("client_assertion: lifetime too long")
	}
	if nbf, ok := payload["nbf"].(float64); ok && now.Unix()+clientAssertionClockSkewSeconds < int64(nbf) {
		return nil, errors.New("client_assertion: not yet valid")
	}
	jti, _ := payload["jti"].(string)
	if jti == "" {
		return nil, errors.New("client_assertion: jti required")
	}

	jwks, err := keysFn(ctx, iss)
	if err != nil {
		return nil, fmt.Errorf("client_assertion: load jwks: %w", err)
	}
	pub, err := pickJWK(jwks, kid)
	if err != nil {
		return nil, err
	}
	if err := verifyJWTSignature(parts, alg, pub); err != nil {
		return nil, fmt.Errorf("client_assertion: signature: %w", err)
	}
	if replay == nil {
		return nil, errors.New("client_assertion: replay store is not configured")
	}
	isNew, err := replay.RecordIfNew(
		ctx,
		jti,
		clientAssertionMaxLifetimeSeconds+clientAssertionClockSkewSeconds,
		now,
	)
	if err != nil {
		return nil, err
	}
	if !isNew {
		return nil, errors.New("client_assertion: jti replay detected")
	}
	return &ClientAssertionResult{ClientID: iss}, nil
}

func verifyAudience(aud any, expected []string) bool {
	allowed := func(candidate string) bool {
		for _, value := range expected {
			if candidate == value {
				return true
			}
		}
		return false
	}
	switch v := aud.(type) {
	case string:
		return allowed(v)
	case []any:
		for _, s := range v {
			if str, ok := s.(string); ok && allowed(str) {
				return true
			}
		}
	}
	return false
}

func pickJWK(jwks []map[string]any, kid string) (any, error) {
	if kid == "" {
		if len(jwks) == 1 {
			return publicKeyFromJWK(jwks[0])
		}
		return nil, errors.New("kid required when JWKS has multiple keys")
	}
	for _, j := range jwks {
		if k, _ := j["kid"].(string); k == kid {
			return publicKeyFromJWK(j)
		}
	}
	return nil, fmt.Errorf("kid %q not found in JWKS", kid)
}
