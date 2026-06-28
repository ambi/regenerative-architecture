package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/infrastructure/crypto"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
)

// KeyStore (OAuth2: 署名鍵)
type KeyStore struct{ Pool *pgxpool.Pool }

func NewKeyStore(ctx context.Context, pool *pgxpool.Pool) (*KeyStore, error) {
	store := &KeyStore{Pool: pool}
	var exists bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM signing_keys WHERE active)").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		if _, err := store.Rotate(ctx); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (s *KeyStore) GetActiveKey(ctx context.Context) (*ports.SigningKey, error) {
	return scanSigningKey(s.Pool.QueryRow(ctx, keySelect+" WHERE active=TRUE LIMIT 1"))
}

func (s *KeyStore) GetAllKeys(ctx context.Context) ([]*ports.SigningKey, error) {
	rows, err := s.Pool.Query(ctx, keySelect+" WHERE archived_at IS NULL ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ports.SigningKey
	for rows.Next() {
		key, err := scanSigningKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *KeyStore) FindByKID(ctx context.Context, kid string) (*ports.SigningKey, error) {
	return scanSigningKey(s.Pool.QueryRow(ctx, keySelect+" WHERE kid=$1", kid))
}

func (s *KeyStore) Rotate(ctx context.Context) (*ports.SigningKey, error) {
	priv, publicJWK, privateJWK, kid, err := crypto.GenerateRSAJWKPair()
	if err != nil {
		return nil, err
	}
	publicJSON, _ := json.Marshal(publicJWK)
	privateJSON, _ := json.Marshal(privateJWK)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('ra-idp-signing-key'))"); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE signing_keys SET active=FALSE,rotated_at=now() WHERE active"); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO signing_keys
(kid,alg,public_jwk,private_jwk,active) VALUES ($1,'PS256',$2,$3,TRUE)`,
		kid, string(publicJSON), string(privateJSON)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &ports.SigningKey{
		Kid: kid, Alg: spec.SigAlgPS256, PrivateKey: priv, PublicKey: &priv.PublicKey,
		PublicJWK: publicJWK, Active: true, CreatedAt: time.Now().UTC(),
	}, nil
}

const keySelect = `SELECT kid,alg,public_jwk,private_jwk,active,created_at FROM signing_keys`

func scanSigningKey(row rowScanner) (*ports.SigningKey, error) {
	var key ports.SigningKey
	var publicJSON, privateJSON []byte
	err := row.Scan(&key.Kid, &key.Alg, &publicJSON, &privateJSON, &key.Active, &key.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var publicJWK, privateJWK map[string]any
	if err := json.Unmarshal(publicJSON, &publicJWK); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(privateJSON, &privateJWK); err != nil {
		return nil, err
	}
	pub, priv, err := crypto.ImportRSAJWK(publicJWK, privateJWK)
	if err != nil {
		return nil, err
	}
	key.PublicJWK, key.PublicKey, key.PrivateKey = publicJWK, pub, priv
	return &key, nil
}
