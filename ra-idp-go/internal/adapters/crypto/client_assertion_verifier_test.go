package crypto

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
)

func signClientAssertion(
	t *testing.T,
	key *rsa.PrivateKey,
	audience, jti string,
	now time.Time,
	ttl time.Duration,
) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "PS256", "kid": "client-key"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"iss": "private-client", "sub": "private-client", "aud": audience, "jti": jti,
		"iat": now.Unix(), "exp": now.Add(ttl).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPSS(
		rand.Reader,
		key,
		crypto.SHA256,
		digest[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyClientAssertion(t *testing.T) {
	const (
		clientID = "private-client"
		audience = "https://idp.example/token"
		kid      = "client-key"
	)
	now := time.Now().UTC()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := rsaPublicJWK(&key.PublicKey)
	jwk["kid"] = kid
	keysFn := func(_ context.Context, requestedClientID string) ([]map[string]any, error) {
		if requestedClientID != clientID {
			return nil, context.Canceled
		}
		return []map[string]any{jwk}, nil
	}

	t.Run("accepts valid assertion and rejects replay", func(t *testing.T) {
		replay := memory.NewClientAssertionReplayStore()
		assertion := signClientAssertion(t, key, audience, "jti-1", now, 2*time.Minute)
		if _, err := VerifyClientAssertion(
			context.Background(), assertion, clientID, []string{audience}, keysFn, replay, now, nil,
		); err != nil {
			t.Fatalf("valid assertion rejected: %v", err)
		}
		if _, err := VerifyClientAssertion(
			context.Background(), assertion, clientID, []string{audience}, keysFn, replay, now, nil,
		); err == nil {
			t.Fatal("expected replay rejection")
		}
	})

	t.Run("rejects wrong audience and excessive lifetime", func(t *testing.T) {
		replay := memory.NewClientAssertionReplayStore()
		wrongAud := signClientAssertion(t, key, "https://other.example", "jti-2", now, time.Minute)
		if _, err := VerifyClientAssertion(
			context.Background(), wrongAud, clientID, []string{audience}, keysFn, replay, now, nil,
		); err == nil {
			t.Fatal("expected audience rejection")
		}
		longLived := signClientAssertion(t, key, audience, "jti-3", now, time.Hour)
		if _, err := VerifyClientAssertion(
			context.Background(), longLived, clientID, []string{audience}, keysFn, replay, now, nil,
		); err == nil {
			t.Fatal("expected lifetime rejection")
		}
	})

	t.Run("invalid signature does not reserve jti", func(t *testing.T) {
		replay := memory.NewClientAssertionReplayStore()
		attacker, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		bad := signClientAssertion(t, attacker, audience, "shared-jti", now, time.Minute)
		if _, err := VerifyClientAssertion(
			context.Background(), bad, clientID, []string{audience}, keysFn, replay, now, nil,
		); err == nil {
			t.Fatal("expected signature rejection")
		}
		good := signClientAssertion(t, key, audience, "shared-jti", now, time.Minute)
		if _, err := VerifyClientAssertion(
			context.Background(), good, clientID, []string{audience}, keysFn, replay, now, nil,
		); err != nil {
			t.Fatalf("valid assertion rejected after bad signature: %v", err)
		}
	})
}
