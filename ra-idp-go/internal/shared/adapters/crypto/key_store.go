// Package crypto: 鍵ストアと JWT 署名 (PS256)。
//
// ローカル開発用 in-memory 鍵ストア。本番では KMS / HSM / Vault を使う想定。
// JWK サムプリント (RFC 7638) を kid として使用する。
package crypto

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"sync"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/shared/spec"
)

func GenerateRSAJWKPair() (*rsa.PrivateKey, map[string]any, map[string]any, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, "", err
	}
	publicJWK := rsaPublicJWK(&priv.PublicKey)
	kid, err := jwkThumbprint(publicJWK)
	if err != nil {
		return nil, nil, nil, "", err
	}
	publicJWK["kid"] = kid
	publicJWK["alg"] = string(spec.SigAlgPS256)
	publicJWK["use"] = "sig"
	privateJWK := map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": string(spec.SigAlgPS256),
		"n":   base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(bigIntFromInt(priv.E)),
		"d":   base64.RawURLEncoding.EncodeToString(priv.D.Bytes()),
		"p":   base64.RawURLEncoding.EncodeToString(priv.Primes[0].Bytes()),
		"q":   base64.RawURLEncoding.EncodeToString(priv.Primes[1].Bytes()),
	}
	return priv, publicJWK, privateJWK, kid, nil
}

func ImportRSAJWK(publicJWK, privateJWK map[string]any) (crypto.PublicKey, crypto.PrivateKey, error) {
	pub, err := publicKeyFromJWK(publicJWK)
	if err != nil {
		return nil, nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("public JWK is not RSA")
	}
	decodeInt := func(name string) (*big.Int, error) {
		value, _ := privateJWK[name].(string)
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(decoded) == 0 {
			return nil, errors.New("private JWK missing or invalid " + name)
		}
		return new(big.Int).SetBytes(decoded), nil
	}
	d, err := decodeInt("d")
	if err != nil {
		return nil, nil, err
	}
	p, err := decodeInt("p")
	if err != nil {
		return nil, nil, err
	}
	q, err := decodeInt("q")
	if err != nil {
		return nil, nil, err
	}
	priv := &rsa.PrivateKey{PublicKey: *rsaPub, D: d, Primes: []*big.Int{p, q}}
	if err := priv.Validate(); err != nil {
		return nil, nil, err
	}
	priv.Precompute()
	return rsaPub, priv, nil
}

type InMemoryKeyStore struct {
	mu     sync.RWMutex
	keys   []*ports.SigningKey
	active string
}

func NewInMemoryKeyStore() (*InMemoryKeyStore, error) {
	ks := &InMemoryKeyStore{}
	if _, err := ks.rotateInternal(); err != nil {
		return nil, err
	}
	return ks, nil
}

func (s *InMemoryKeyStore) GetActiveKey(_ context.Context) (*ports.SigningKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		if k.Kid == s.active {
			return k, nil
		}
	}
	return nil, errors.New("no active signing key")
}

func (s *InMemoryKeyStore) GetAllKeys(_ context.Context) ([]*ports.SigningKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*ports.SigningKey, len(s.keys))
	copy(out, s.keys)
	return out, nil
}

func (s *InMemoryKeyStore) FindByKID(_ context.Context, kid string) (*ports.SigningKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		if k.Kid == kid {
			return k, nil
		}
	}
	return nil, nil
}

func (s *InMemoryKeyStore) Rotate(_ context.Context) (*ports.SigningKey, error) {
	return s.rotateInternal()
}

func (s *InMemoryKeyStore) rotateInternal() (*ports.SigningKey, error) {
	priv, jwk, _, kid, err := GenerateRSAJWKPair()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		k.Active = false
	}
	key := &ports.SigningKey{
		Kid:        kid,
		Alg:        spec.SigAlgPS256,
		PrivateKey: priv,
		PublicKey:  &priv.PublicKey,
		PublicJWK:  jwk,
		Active:     true,
		CreatedAt:  time.Now().UTC(),
	}
	s.keys = append(s.keys, key)
	s.active = kid
	return key, nil
}

// rsaPublicJWK は RSA 公開鍵を JWK 形式の map に変換する。
func rsaPublicJWK(pub *rsa.PublicKey) map[string]any {
	return map[string]any{
		"kty": "RSA",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(bigIntFromInt(pub.E)),
	}
}

// bigIntFromInt は RSA 公開指数 (E) 等の非負整数を big-endian bytes に符号化する。
// big.Int 経由で先頭の 0x00 を取り除き、JWK の "e" メンバー形式に合わせる。
func bigIntFromInt(v int) []byte {
	return new(big.Int).SetInt64(int64(v)).Bytes()
}

// jwkThumbprint は RFC 7638 に従い JWK の SHA-256 サムプリントを base64url で返す。
// canonical JSON: required メンバーのみ、辞書順、空白なし。
func jwkThumbprint(jwk map[string]any) (string, error) {
	required := []string{"e", "kty", "n"}
	canon := map[string]any{}
	for _, k := range required {
		v, ok := jwk[k]
		if !ok {
			return "", errors.New("jwk missing required member: " + k)
		}
		canon[k] = v
	}
	b, err := json.Marshal(canon)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
