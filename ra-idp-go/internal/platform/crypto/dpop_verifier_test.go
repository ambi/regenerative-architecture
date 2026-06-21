package crypto

// DPoP proof JWT (RFC 9449) の表駆動ユニットテスト。
// VerifyDPoP の典型的な失敗ケース (typ/alg/iat/jti/htm/htu) と happy path を網羅する。

import (
	"context"
	cryptostd "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ra-idp-go/internal/adapters/persistence/memory"
)

func dpopTestKey(t *testing.T) (*rsa.PrivateKey, map[string]any) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key, rsaPublicJWK(&key.PublicKey)
}

func encodeDPoPProof(t *testing.T, key *rsa.PrivateKey, header, payload map[string]any) string {
	t.Helper()
	hb, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." +
		base64.RawURLEncoding.EncodeToString(pb)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPSS(
		rand.Reader, key, cryptostd.SHA256, digest[:],
		&rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash},
	)
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestVerifyDPoPAcceptsValidProof(t *testing.T) {
	key, jwk := dpopTestKey(t)
	now := time.Now().UTC()
	proof := encodeDPoPProof(t, key,
		map[string]any{"typ": "dpop+jwt", "alg": "PS256", "jwk": jwk},
		map[string]any{"htm": "POST", "htu": "https://idp.example/token", "jti": "jti-ok", "iat": now.Unix()},
	)
	res, err := VerifyDPoP(
		context.Background(), proof, "POST", "https://idp.example/token",
		memory.NewDpopReplayStore(), now,
	)
	if err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
	if res == nil || res.JKT == "" {
		t.Fatalf("expected non-empty thumbprint, got %+v", res)
	}
	expectedJKT, err := jwkThumbprint(jwk)
	if err != nil {
		t.Fatal(err)
	}
	if res.JKT != expectedJKT {
		t.Fatalf("jkt mismatch got=%s want=%s", res.JKT, expectedJKT)
	}
}

func TestVerifyDPoPRejectsFailureCases(t *testing.T) {
	key, jwk := dpopTestKey(t)
	now := time.Now().UTC()
	const validHTU = "https://idp.example/token"

	type proofMutator func(header, payload map[string]any)
	cases := []struct {
		name      string
		mutate    proofMutator
		wantError string // substring of expected error
		htm       string
		htu       string
	}{
		{
			name:      "typ が dpop+jwt でない",
			mutate:    func(h, _ map[string]any) { h["typ"] = "jwt" },
			wantError: "typ must be dpop+jwt",
		},
		{
			name:      "alg が PS256 / ES256 以外",
			mutate:    func(h, _ map[string]any) { h["alg"] = "HS256" },
			wantError: "alg must be",
		},
		{
			name:      "jwk header が欠落",
			mutate:    func(h, _ map[string]any) { delete(h, "jwk") },
			wantError: "jwk header required",
		},
		{
			name:      "iat が clock skew より過去",
			mutate:    func(_, p map[string]any) { p["iat"] = now.Add(-2 * time.Minute).Unix() },
			wantError: "iat outside",
		},
		{
			name:      "iat が clock skew より未来",
			mutate:    func(_, p map[string]any) { p["iat"] = now.Add(time.Minute).Unix() },
			wantError: "iat outside",
		},
		{
			name:      "jti 欠落",
			mutate:    func(_, p map[string]any) { delete(p, "jti") },
			wantError: "jti required",
		},
		{
			name:      "htm 不一致",
			mutate:    func(_, p map[string]any) { p["htm"] = "GET" },
			wantError: "htm mismatch",
		},
		{
			name:      "htu 不一致",
			mutate:    func(_, p map[string]any) { p["htu"] = "https://idp.example/other" },
			wantError: "htu mismatch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			header := map[string]any{"typ": "dpop+jwt", "alg": "PS256", "jwk": jwk}
			payload := map[string]any{"htm": "POST", "htu": validHTU, "jti": tc.name, "iat": now.Unix()}
			tc.mutate(header, payload)
			proof := encodeDPoPProof(t, key, header, payload)
			_, err := VerifyDPoP(
				context.Background(), proof, "POST", validHTU,
				memory.NewDpopReplayStore(), now,
			)
			if err == nil {
				t.Fatalf("expected rejection containing %q", tc.wantError)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantError)
			}
		})
	}
}

func TestVerifyDPoPDetectsReplay(t *testing.T) {
	// 同一 jti の再使用は ReplayWindow 内で拒否される (DpopJtiUniquenessWithinWindow)。
	key, jwk := dpopTestKey(t)
	now := time.Now().UTC()
	store := memory.NewDpopReplayStore()
	proof := encodeDPoPProof(t, key,
		map[string]any{"typ": "dpop+jwt", "alg": "PS256", "jwk": jwk},
		map[string]any{"htm": "POST", "htu": "https://idp.example/token", "jti": "replay-jti", "iat": now.Unix()},
	)
	if _, err := VerifyDPoP(context.Background(), proof, "POST", "https://idp.example/token", store, now); err != nil {
		t.Fatalf("first attempt rejected: %v", err)
	}
	_, err := VerifyDPoP(context.Background(), proof, "POST", "https://idp.example/token", store, now)
	if err == nil || !strings.Contains(err.Error(), "replay") {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestVerifyDPoPRejectsInvalidSignature(t *testing.T) {
	// 別鍵で署名された proof は jwk header と一致しないため署名検証で落ちる。
	signer, _ := dpopTestKey(t)
	_, claimedJWK := dpopTestKey(t)
	now := time.Now().UTC()
	proof := encodeDPoPProof(t, signer,
		map[string]any{"typ": "dpop+jwt", "alg": "PS256", "jwk": claimedJWK},
		map[string]any{"htm": "POST", "htu": "https://idp.example/token", "jti": "sig-mismatch", "iat": now.Unix()},
	)
	_, err := VerifyDPoP(
		context.Background(), proof, "POST", "https://idp.example/token",
		memory.NewDpopReplayStore(), now,
	)
	if err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("expected signature rejection, got %v", err)
	}
}

func TestVerifyDPoPMissingHeaderIsNoOp(t *testing.T) {
	// 空ヘッダーは「proof 不在」を意味する。エラー無しで nil を返し、呼び出し側の責務に委ねる。
	res, err := VerifyDPoP(
		context.Background(), "", "POST", "https://idp.example/token",
		memory.NewDpopReplayStore(), time.Now().UTC(),
	)
	if err != nil || res != nil {
		t.Fatalf("empty header: got res=%v err=%v, want nil/nil", res, err)
	}
}
