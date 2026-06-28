// Package crypto: Layer 4 - Adapter Layer (crypto: Argon2id password hasher)
//
// golang.org/x/crypto/argon2 の IDKey を用い、OWASP 2024 推奨パラメータ
// (m=19456 KiB, t=2, p=1, keyLen=32, saltLen=16) で PHC 形式の文字列を生成する。
// 形式は Bun.password と相互運用可能:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<base64-salt>$<base64-hash>
//
// Verify は PHC 文字列に埋め込まれたパラメータでデコードするため、将来パラメータを
// 更新しても旧 hash と互換性を保つ（hash 側だけ新パラメータで再生成すればよい）。
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	owaspMemoryCostKiB uint32 = 19456
	owaspTimeCost      uint32 = 2
	owaspParallelism   uint8  = 1
	keyLen             uint32 = 32
	saltLen                   = 16
)

type Argon2idPasswordHasher struct {
	MemoryCost  uint32
	TimeCost    uint32
	Parallelism uint8
}

func NewArgon2idPasswordHasher() *Argon2idPasswordHasher {
	return &Argon2idPasswordHasher{
		MemoryCost:  owaspMemoryCostKiB,
		TimeCost:    owaspTimeCost,
		Parallelism: owaspParallelism,
	}
}

func (h *Argon2idPasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2id: read salt: %w", err)
	}
	digest := argon2.IDKey([]byte(password), salt, h.TimeCost, h.MemoryCost, h.Parallelism, keyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.MemoryCost, h.TimeCost, h.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

func (h *Argon2idPasswordHasher) Verify(password, encoded string) (bool, error) {
	params, salt, digest, err := decodePHC(encoded)
	if err != nil {
		return false, err
	}
	// len(digest) は keyLen (=32) と同じスケールで MaxUint32 を超えない。
	candidate := argon2.IDKey([]byte(password), salt, params.timeCost, params.memoryCost, params.parallelism, uint32(len(digest))) //nolint:gosec // len bound by keyLen
	return subtle.ConstantTimeCompare(candidate, digest) == 1, nil
}

type argon2idParams struct {
	memoryCost  uint32
	timeCost    uint32
	parallelism uint8
}

// decodePHC は $argon2id$v=19$m=...,t=...,p=...$<b64salt>$<b64hash> 形式を解く。
func decodePHC(encoded string) (argon2idParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// 先頭が空文字なので 6 要素: ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 {
		return argon2idParams{}, nil, nil, errors.New("argon2id: malformed PHC string")
	}
	if parts[1] != "argon2id" {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: unexpected algorithm %q", parts[1])
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: parse version: %w", err)
	}
	if version != argon2.Version {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: unsupported version %d", version)
	}
	var p argon2idParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryCost, &p.timeCost, &p.parallelism); err != nil {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: decode salt: %w", err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2idParams{}, nil, nil, fmt.Errorf("argon2id: decode digest: %w", err)
	}
	return p, salt, digest, nil
}
