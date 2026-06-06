// Package crypto: DPoP proof JWT 検証 (RFC 9449)。
package crypto

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"ra-idp-go/internal/oauth2/ports"
)

const (
	dpopClockSkewPastSeconds   = 60
	dpopClockSkewFutureSeconds = 5
	dpopJTIReplayWindowSeconds = 600
)

type DPoPResult struct {
	JKT string
}

// VerifyDPoP は DPoP ヘッダ JWT を検証して JWK thumbprint を返す。
// 期待 htm/htu と乖離した場合 / 署名検証失敗 / iat スキュー外 / jti リプレイで error。
func VerifyDPoP(
	ctx context.Context,
	dpopHeader, expectedHTM, expectedHTU string,
	replay ports.DpopReplayStore,
	now time.Time,
) (*DPoPResult, error) {
	if dpopHeader == "" {
		return nil, nil
	}
	parts := strings.Split(dpopHeader, ".")
	if len(parts) != 3 {
		return nil, errors.New("dpop: malformed proof")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("dpop: decode header: %w", err)
	}
	var header map[string]any
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, fmt.Errorf("dpop: parse header: %w", err)
	}
	if typ, _ := header["typ"].(string); typ != "dpop+jwt" {
		return nil, errors.New("dpop: typ must be dpop+jwt")
	}
	alg, _ := header["alg"].(string)
	if alg != "PS256" && alg != "ES256" {
		return nil, errors.New("dpop: alg must be PS256 or ES256")
	}
	jwk, _ := header["jwk"].(map[string]any)
	if jwk == nil {
		return nil, errors.New("dpop: jwk header required")
	}

	pub, err := publicKeyFromJWK(jwk)
	if err != nil {
		return nil, fmt.Errorf("dpop: import jwk: %w", err)
	}
	if err := verifyJWTSignature(parts, alg, pub); err != nil {
		return nil, fmt.Errorf("dpop: signature: %w", err)
	}

	pb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("dpop: decode payload: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(pb, &payload); err != nil {
		return nil, fmt.Errorf("dpop: parse payload: %w", err)
	}
	if htm, _ := payload["htm"].(string); htm != expectedHTM {
		return nil, fmt.Errorf("dpop: htm mismatch (got %q, want %q)", htm, expectedHTM)
	}
	if htu, _ := payload["htu"].(string); htu != expectedHTU {
		return nil, fmt.Errorf("dpop: htu mismatch (got %q, want %q)", htu, expectedHTU)
	}
	iat, _ := payload["iat"].(float64)
	skew := now.Unix() - int64(iat)
	if skew > dpopClockSkewPastSeconds || skew < -dpopClockSkewFutureSeconds {
		return nil, fmt.Errorf("dpop: iat outside clock skew")
	}
	jti, _ := payload["jti"].(string)
	if jti == "" {
		return nil, errors.New("dpop: jti required")
	}
	isNew, err := replay.RecordIfNew(ctx, jti, dpopJTIReplayWindowSeconds, now)
	if err != nil {
		return nil, err
	}
	if !isNew {
		return nil, errors.New("dpop: jti replay detected")
	}
	jkt, err := jwkThumbprint(jwk)
	if err != nil {
		return nil, err
	}
	return &DPoPResult{JKT: jkt}, nil
}

// =====================================================================
// JWK → 公開鍵
// =====================================================================

func publicKeyFromJWK(jwk map[string]any) (crypto.PublicKey, error) {
	kty, _ := jwk["kty"].(string)
	switch kty {
	case "RSA":
		nB64, _ := jwk["n"].(string)
		eB64, _ := jwk["e"].(string)
		nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
		if err != nil {
			return nil, err
		}
		n := new(big.Int).SetBytes(nBytes)
		e := 0
		for _, b := range eBytes {
			e = (e << 8) | int(b)
		}
		return &rsa.PublicKey{N: n, E: e}, nil
	case "EC":
		crv, _ := jwk["crv"].(string)
		if crv != "P-256" {
			return nil, fmt.Errorf("unsupported EC curve %q", crv)
		}
		xB64, _ := jwk["x"].(string)
		yB64, _ := jwk["y"].(string)
		xBytes, err := base64.RawURLEncoding.DecodeString(xB64)
		if err != nil {
			return nil, err
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(yB64)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}, nil
	}
	return nil, fmt.Errorf("unsupported kty %q", kty)
}

// verifyJWTSignature は alg ごとに署名を検証する。
func verifyJWTSignature(parts []string, alg string, pub crypto.PublicKey) error {
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	switch alg {
	case "PS256":
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errors.New("PS256 requires RSA public key")
		}
		return rsa.VerifyPSS(rsaPub, crypto.SHA256, digest[:], sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	case "ES256":
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("ES256 requires EC public key")
		}
		if len(sig) != 64 {
			return errors.New("ES256 signature must be 64 bytes")
		}
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if !ecdsa.Verify(ecPub, digest[:], r, s) {
			return errors.New("ES256 signature invalid")
		}
		return nil
	}
	return fmt.Errorf("unsupported alg %q", alg)
}
