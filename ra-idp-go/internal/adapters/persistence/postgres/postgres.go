package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ra-idp-go/internal/adapters/crypto"
	authports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MaxConnIdleTime = 30 * time.Second
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET statement_timeout = '5s'; SET idle_in_transaction_session_timeout = '30s'")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type TenantRepository struct{ Pool *pgxpool.Pool }

func (r *TenantRepository) FindByID(ctx context.Context, id string) (*spec.Tenant, error) {
	return scanTenant(r.Pool.QueryRow(ctx, tenantSelect+" WHERE id=$1", id))
}

func (r *TenantRepository) FindAll(ctx context.Context) ([]*spec.Tenant, error) {
	rows, err := r.Pool.Query(ctx, tenantSelect+" ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.Tenant{}
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tenant)
	}
	return out, rows.Err()
}

func (r *TenantRepository) Save(ctx context.Context, tenant *spec.Tenant) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO tenants
(id,display_name,status,created_at,updated_at,disabled_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name,
status=EXCLUDED.status,updated_at=EXCLUDED.updated_at,disabled_at=EXCLUDED.disabled_at`,
		tenant.ID, tenant.DisplayName, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt, tenant.DisabledAt)
	return err
}

const tenantSelect = `SELECT id,display_name,status,created_at,updated_at,disabled_at FROM tenants`

func scanTenant(row rowScanner) (*spec.Tenant, error) {
	var tenant spec.Tenant
	err := row.Scan(&tenant.ID, &tenant.DisplayName, &tenant.Status, &tenant.CreatedAt,
		&tenant.UpdatedAt, &tenant.DisabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tenant, tenant.Validate()
}

type ClientRepository struct{ Pool *pgxpool.Pool }

func (r *ClientRepository) FindByID(ctx context.Context, tenantID, id string) (*spec.Client, error) {
	row := r.Pool.QueryRow(ctx, clientSelect+" WHERE tenant_id=$1 AND client_id=$2", tenantID, id)
	return scanClient(row)
}

func (r *ClientRepository) Save(ctx context.Context, c *spec.Client) error {
	redirectURIs, _ := json.Marshal(c.RedirectURIs)
	grantTypes, _ := json.Marshal(c.GrantTypes)
	responseTypes, _ := json.Marshal(c.ResponseTypes)
	jwks, _ := json.Marshal(c.JWKS)
	_, err := r.Pool.Exec(ctx, `
INSERT INTO clients (
 tenant_id,client_id,client_secret_hash,client_name,client_type,redirect_uris,grant_types,response_types,
 token_endpoint_auth_method,scope,jwks_uri,jwks,tls_client_auth_subject_dn,
 id_token_signed_response_alg,require_pushed_authorization_requests,dpop_bound_access_tokens,
 fapi_profile,created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,'null')::jsonb,$13,$14,$15,$16,$17,$18)
ON CONFLICT (tenant_id,client_id) DO UPDATE SET
 client_secret_hash=COALESCE(EXCLUDED.client_secret_hash,clients.client_secret_hash),
 client_name=EXCLUDED.client_name,client_type=EXCLUDED.client_type,
 redirect_uris=EXCLUDED.redirect_uris,grant_types=EXCLUDED.grant_types,
 response_types=EXCLUDED.response_types,token_endpoint_auth_method=EXCLUDED.token_endpoint_auth_method,
 scope=EXCLUDED.scope,jwks_uri=EXCLUDED.jwks_uri,jwks=EXCLUDED.jwks,
 tls_client_auth_subject_dn=EXCLUDED.tls_client_auth_subject_dn,
 id_token_signed_response_alg=EXCLUDED.id_token_signed_response_alg,
 require_pushed_authorization_requests=EXCLUDED.require_pushed_authorization_requests,
 dpop_bound_access_tokens=EXCLUDED.dpop_bound_access_tokens,fapi_profile=EXCLUDED.fapi_profile`,
		c.TenantID, c.ClientID, c.ClientSecretHash, c.ClientName, c.ClientType, string(redirectURIs), string(grantTypes),
		string(responseTypes), c.TokenEndpointAuthMethod, c.Scope, c.JwksURI, string(jwks),
		c.TlsClientAuthSubjectDN, c.IDTokenSignedResponseAlg,
		c.RequirePushedAuthorizationRequests, c.DpopBoundAccessTokens, c.FapiProfile, c.CreatedAt)
	return err
}

func (r *ClientRepository) Delete(ctx context.Context, tenantID, id string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM clients WHERE tenant_id=$1 AND client_id=$2", tenantID, id)
	return err
}

func (r *ClientRepository) FindAll(ctx context.Context, tenantID string) ([]*spec.Client, error) {
	rows, err := r.Pool.Query(ctx, clientSelect+" WHERE tenant_id=$1 ORDER BY created_at", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*spec.Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, client)
	}
	return out, rows.Err()
}

const clientSelect = `SELECT tenant_id,client_id,client_secret_hash,client_name,client_type,redirect_uris,
grant_types,response_types,token_endpoint_auth_method,scope,jwks_uri,jwks,
tls_client_auth_subject_dn,id_token_signed_response_alg,
require_pushed_authorization_requests,dpop_bound_access_tokens,fapi_profile,created_at FROM clients`

type rowScanner interface{ Scan(...any) error }

func scanClient(row rowScanner) (*spec.Client, error) {
	var c spec.Client
	var redirectURIs, grantTypes, responseTypes, jwks []byte
	err := row.Scan(&c.TenantID, &c.ClientID, &c.ClientSecretHash, &c.ClientName, &c.ClientType,
		&redirectURIs, &grantTypes, &responseTypes, &c.TokenEndpointAuthMethod, &c.Scope,
		&c.JwksURI, &jwks, &c.TlsClientAuthSubjectDN, &c.IDTokenSignedResponseAlg,
		&c.RequirePushedAuthorizationRequests, &c.DpopBoundAccessTokens, &c.FapiProfile, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(redirectURIs, &c.RedirectURIs); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(grantTypes, &c.GrantTypes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(responseTypes, &c.ResponseTypes); err != nil {
		return nil, err
	}
	if len(jwks) > 0 {
		if err := json.Unmarshal(jwks, &c.JWKS); err != nil {
			return nil, err
		}
	}
	return &c, c.Validate()
}

type UserRepository struct{ Pool *pgxpool.Pool }

func (r *UserRepository) FindBySub(ctx context.Context, sub string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE sub=$1 AND deleted_at IS NULL", sub))
}

func (r *UserRepository) FindBySubIncludingDeleted(ctx context.Context, sub string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE sub=$1", sub))
}

func (r *UserRepository) FindByUsername(ctx context.Context, tenantID, username string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(ctx, userSelect+" WHERE tenant_id=$1 AND preferred_username=$2 AND deleted_at IS NULL", tenantID, username))
}

func (r *UserRepository) FindByEmail(ctx context.Context, tenantID, email string) (*spec.User, error) {
	return scanUser(r.Pool.QueryRow(
		ctx,
		userSelect+" WHERE tenant_id=$1 AND lower(email)=lower($2) AND deleted_at IS NULL LIMIT 1",
		tenantID, email,
	))
}

func (r *UserRepository) FindAll(ctx context.Context, tenantID string) ([]*spec.User, error) {
	rows, err := r.Pool.Query(ctx, userSelect+" WHERE tenant_id=$1 AND deleted_at IS NULL ORDER BY preferred_username", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*spec.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) Save(ctx context.Context, u *spec.User) error {
	// lifecycle / attributes は JSONB に格納する (ADR-039)。多値属性は本 PR では
	// 単一カラムで持ち、検索が要るようになった段階で別テーブル化する。
	lifecycle, err := json.Marshal(u.Lifecycle)
	if err != nil {
		return err
	}
	attributes, err := json.Marshal(u.Attributes)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO users (sub,tenant_id,preferred_username,password_hash,name,given_name,family_name,email,
 email_verified,mfa_enrolled,roles,lifecycle,attributes,created_at,updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (sub) DO UPDATE SET preferred_username=EXCLUDED.preferred_username,
 password_hash=EXCLUDED.password_hash,name=EXCLUDED.name,given_name=EXCLUDED.given_name,
 family_name=EXCLUDED.family_name,email=EXCLUDED.email,email_verified=EXCLUDED.email_verified,
 mfa_enrolled=EXCLUDED.mfa_enrolled,roles=EXCLUDED.roles,lifecycle=EXCLUDED.lifecycle,
 attributes=EXCLUDED.attributes,updated_at=EXCLUDED.updated_at`,
		u.Sub, u.TenantID, u.PreferredUsername, u.PasswordHash, u.Name, u.GivenName, u.FamilyName, u.Email,
		u.EmailVerified, u.MfaEnrolled, u.Roles, lifecycle, attributes, u.CreatedAt, u.UpdatedAt)
	return err
}

const userSelect = `SELECT sub,tenant_id,preferred_username,password_hash,name,given_name,family_name,email,
email_verified,mfa_enrolled,roles,lifecycle,attributes,created_at,updated_at FROM users`

func scanUser(row rowScanner) (*spec.User, error) {
	var u spec.User
	var lifecycle, attributes []byte
	err := row.Scan(&u.Sub, &u.TenantID, &u.PreferredUsername, &u.PasswordHash, &u.Name, &u.GivenName,
		&u.FamilyName, &u.Email, &u.EmailVerified, &u.MfaEnrolled, &u.Roles, &lifecycle, &attributes,
		&u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(lifecycle) > 0 {
		if err := json.Unmarshal(lifecycle, &u.Lifecycle); err != nil {
			return nil, err
		}
	}
	if len(attributes) > 0 {
		if err := json.Unmarshal(attributes, &u.Attributes); err != nil {
			return nil, err
		}
	}
	return &u, u.Validate()
}

type PasswordHistoryRepository struct{ Pool *pgxpool.Pool }

func (r *PasswordHistoryRepository) Recent(
	ctx context.Context,
	sub string,
	depth int,
) ([]authports.PasswordHistoryEntry, error) {
	if depth <= 0 {
		return nil, nil
	}
	rows, err := r.Pool.Query(ctx, `SELECT encoded,created_at
FROM password_history
WHERE sub=$1
ORDER BY created_at DESC, id DESC
LIMIT $2`, sub, depth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authports.PasswordHistoryEntry{}
	for rows.Next() {
		var entry authports.PasswordHistoryEntry
		if err := rows.Scan(&entry.Encoded, &entry.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *PasswordHistoryRepository) Add(ctx context.Context, sub, encoded string, now time.Time) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO password_history (sub,encoded,created_at) VALUES ($1,$2,$3)`,
		sub, encoded, now)
	return err
}

func (r *PasswordHistoryRepository) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM password_history WHERE sub=$1", sub)
	return err
}

type PasswordResetTokenStore struct{ Pool *pgxpool.Pool }

func (s *PasswordResetTokenStore) Save(
	ctx context.Context,
	record authports.PasswordResetTokenRecord,
) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "DELETE FROM password_reset_tokens WHERE sub=$1", record.Sub); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO password_reset_tokens
(token_hash,sub,created_at,expires_at) VALUES ($1,$2,$3,$4)`,
		record.TokenHash, record.Sub, record.CreatedAt, record.ExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PasswordResetTokenStore) Consume(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) (*authports.PasswordResetTokenRecord, error) {
	var record authports.PasswordResetTokenRecord
	err := s.Pool.QueryRow(ctx, `DELETE FROM password_reset_tokens
WHERE token_hash=$1
RETURNING sub,token_hash,created_at,expires_at`, tokenHash).
		Scan(&record.Sub, &record.TokenHash, &record.CreatedAt, &record.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !now.Before(record.ExpiresAt) {
		return nil, nil
	}
	return &record, nil
}

type MfaFactorRepository struct{ Pool *pgxpool.Pool }

func (r *MfaFactorRepository) ListBySub(ctx context.Context, sub string) ([]*spec.MfaFactor, error) {
	rows, err := r.Pool.Query(ctx, mfaFactorSelect+" WHERE sub=$1 ORDER BY created_at", sub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.MfaFactor{}
	for rows.Next() {
		factor, err := scanMfaFactor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, factor)
	}
	return out, rows.Err()
}

func (r *MfaFactorRepository) Find(
	ctx context.Context,
	sub string,
	factorType spec.MfaFactorType,
) (*spec.MfaFactor, error) {
	return scanMfaFactor(r.Pool.QueryRow(ctx, mfaFactorSelect+" WHERE sub=$1 AND type=$2", sub, factorType))
}

func (r *MfaFactorRepository) Save(ctx context.Context, factor *spec.MfaFactor) error {
	_, err := r.Pool.Exec(ctx, `
INSERT INTO mfa_factors (sub,type,secret,label,created_at,last_used_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (sub,type) DO UPDATE SET secret=EXCLUDED.secret,label=EXCLUDED.label,last_used_at=EXCLUDED.last_used_at`,
		factor.Sub, factor.Type, factor.Secret, factor.Label, factor.CreatedAt, factor.LastUsedAt)
	return err
}

func (r *MfaFactorRepository) Delete(ctx context.Context, sub string, factorType spec.MfaFactorType) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM mfa_factors WHERE sub=$1 AND type=$2", sub, factorType)
	return err
}

func (r *MfaFactorRepository) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM mfa_factors WHERE sub=$1", sub)
	return err
}

const mfaFactorSelect = `SELECT sub,type,secret,label,created_at,last_used_at FROM mfa_factors`

func scanMfaFactor(row rowScanner) (*spec.MfaFactor, error) {
	var factor spec.MfaFactor
	err := row.Scan(&factor.Sub, &factor.Type, &factor.Secret, &factor.Label, &factor.CreatedAt, &factor.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &factor, factor.Validate()
}

type ConsentRepository struct{ Pool *pgxpool.Pool }

func (r *ConsentRepository) Find(ctx context.Context, tenantID, sub, clientID string) (*spec.Consent, error) {
	var c spec.Consent
	var scopes []byte
	err := r.Pool.QueryRow(ctx, `SELECT tenant_id,sub,client_id,scopes,granted_at,expires_at,revoked_at
FROM consents WHERE tenant_id=$1 AND sub=$2 AND client_id=$3`, tenantID, sub, clientID).
		Scan(&c.TenantID, &c.Sub, &c.ClientID, &scopes, &c.GrantedAt, &c.ExpiresAt, &c.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(scopes, &c.Scopes); err != nil {
		return nil, err
	}
	switch {
	case c.RevokedAt != nil:
		c.State = spec.ConsentRevoked
	case !time.Now().Before(c.ExpiresAt):
		c.State = spec.ConsentExpired
	default:
		c.State = spec.ConsentGranted
	}
	return &c, nil
}

func (r *ConsentRepository) FindAll(ctx context.Context, tenantID string) ([]*spec.Consent, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id,sub,client_id,scopes,granted_at,expires_at,revoked_at
FROM consents WHERE tenant_id=$1 ORDER BY sub,client_id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var consents []*spec.Consent
	now := time.Now()
	for rows.Next() {
		var consent spec.Consent
		var scopes []byte
		if err := rows.Scan(
			&consent.TenantID, &consent.Sub, &consent.ClientID, &scopes,
			&consent.GrantedAt, &consent.ExpiresAt, &consent.RevokedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(scopes, &consent.Scopes); err != nil {
			return nil, err
		}
		switch {
		case consent.RevokedAt != nil:
			consent.State = spec.ConsentRevoked
		case !now.Before(consent.ExpiresAt):
			consent.State = spec.ConsentExpired
		default:
			consent.State = spec.ConsentGranted
		}
		consents = append(consents, &consent)
	}
	return consents, rows.Err()
}

func (r *ConsentRepository) Save(ctx context.Context, c *spec.Consent) error {
	scopes, _ := json.Marshal(c.Scopes)
	_, err := r.Pool.Exec(ctx, `INSERT INTO consents
(tenant_id,sub,client_id,scopes,granted_at,expires_at,revoked_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (tenant_id,sub,client_id) DO UPDATE SET scopes=EXCLUDED.scopes,
granted_at=EXCLUDED.granted_at,expires_at=EXCLUDED.expires_at,revoked_at=EXCLUDED.revoked_at`,
		c.TenantID, c.Sub, c.ClientID, string(scopes), c.GrantedAt, c.ExpiresAt, c.RevokedAt)
	return err
}

func (r *ConsentRepository) Revoke(ctx context.Context, tenantID, sub, clientID string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE consents SET revoked_at=now()
WHERE tenant_id=$1 AND sub=$2 AND client_id=$3 AND revoked_at IS NULL`, tenantID, sub, clientID)
	return err
}

func (r *ConsentRepository) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := r.Pool.Exec(ctx, "DELETE FROM consents WHERE sub=$1", sub)
	return err
}

type RefreshTokenStore struct{ Pool *pgxpool.Pool }

func (s *RefreshTokenStore) FindByHash(ctx context.Context, hash string) (*spec.RefreshTokenRecord, error) {
	return scanRefresh(s.Pool.QueryRow(ctx, refreshSelect+" WHERE hash=$1", hash))
}

func (s *RefreshTokenStore) Save(ctx context.Context, rec *spec.RefreshTokenRecord) error {
	return insertRefresh(ctx, s.Pool, rec)
}

func (s *RefreshTokenStore) Rotate(ctx context.Context, parentID string, next *spec.RefreshTokenRecord) (*spec.RefreshTokenRecord, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var rotated, revoked bool
	err = tx.QueryRow(ctx, `SELECT rotated,revoked FROM refresh_tokens WHERE id=$1 FOR UPDATE`, parentID).
		Scan(&rotated, &revoked)
	if errors.Is(err, pgx.ErrNoRows) || rotated || revoked {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "UPDATE refresh_tokens SET rotated=TRUE WHERE id=$1", parentID); err != nil {
		return nil, err
	}
	if err := insertRefresh(ctx, tx, next); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *RefreshTokenStore) RevokeFamily(ctx context.Context, familyID string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE refresh_tokens SET revoked=TRUE WHERE family_id=$1", familyID)
	return err
}

func (s *RefreshTokenStore) DeleteAllForSub(ctx context.Context, sub string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM refresh_tokens WHERE sub=$1", sub)
	return err
}

const refreshSelect = `SELECT id::text,tenant_id,hash,family_id::text,parent_id::text,client_id,sub,scopes,
issued_at,expires_at,absolute_expires_at,revoked,rotated,sender_constraint FROM refresh_tokens`

func scanRefresh(row rowScanner) (*spec.RefreshTokenRecord, error) {
	var rec spec.RefreshTokenRecord
	var parentID *string
	var scopes, constraint []byte
	err := row.Scan(&rec.ID, &rec.TenantID, &rec.Hash, &rec.FamilyID, &parentID, &rec.ClientID, &rec.Sub,
		&scopes, &rec.IssuedAt, &rec.ExpiresAt, &rec.AbsoluteExpiresAt, &rec.Revoked,
		&rec.Rotated, &constraint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.ParentID = parentID
	if err := json.Unmarshal(scopes, &rec.Scopes); err != nil {
		return nil, err
	}
	if len(constraint) > 0 {
		if err := json.Unmarshal(constraint, &rec.SenderConstraint); err != nil {
			return nil, err
		}
	}
	return &rec, rec.Validate()
}

func insertRefresh(ctx context.Context, db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, rec *spec.RefreshTokenRecord,
) error {
	scopes, _ := json.Marshal(rec.Scopes)
	constraint, _ := json.Marshal(rec.SenderConstraint)
	_, err := db.Exec(ctx, `INSERT INTO refresh_tokens
(id,tenant_id,hash,family_id,parent_id,client_id,sub,scopes,issued_at,expires_at,absolute_expires_at,
revoked,rotated,sender_constraint) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,'null')::jsonb)`,
		rec.ID, rec.TenantID, rec.Hash, rec.FamilyID, rec.ParentID, rec.ClientID, rec.Sub, string(scopes),
		rec.IssuedAt, rec.ExpiresAt, rec.AbsoluteExpiresAt, rec.Revoked, rec.Rotated, string(constraint))
	return err
}

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

var eventTopics = map[string]string{
	"ClientRegistered": "oauth2.client.lifecycle.v1", "UserAuthenticated": "oauth2.authentication.v1",
	"AuthenticationFailed": "oauth2.authentication.v1", "LoginThrottled": "oauth2.security-incident.v1",
	"PasswordChanged": "oauth2.authentication.v1", "ConsentGranted": "oauth2.consent.v1",
	"ConsentRevoked": "oauth2.consent.v1", "AuthorizationCodeIssued": "oauth2.authorization-code.v1",
	"AuthorizationCodeRedeemed": "oauth2.authorization-code.v1", "AccessTokenIssued": "oauth2.token.v1",
	"RefreshTokenIssued": "oauth2.token.v1", "RefreshTokenRotated": "oauth2.token.v1",
	"TokenRevoked": "oauth2.token.v1", "TokenIntrospected": "oauth2.token.v1",
	"RefreshTokenReuseDetected": "oauth2.security-incident.v1", "SigningKeyRotated": "oauth2.key-management.v1",
	"PARStored": "oauth2.par.v1", "DeviceAuthorizationRequested": "oauth2.device-authorization.v1",
	"DeviceAuthorizationApproved": "oauth2.device-authorization.v1", "DeviceAuthorizationDenied": "oauth2.device-authorization.v1",
	"TenantCreated": "tenancy.lifecycle.v1", "TenantUpdated": "tenancy.lifecycle.v1",
	"TenantDisabled": "tenancy.lifecycle.v1", "TenantEnabled": "tenancy.lifecycle.v1",
	"AdminClientCreated": "oauth2.administration.v1", "AdminClientUpdated": "oauth2.administration.v1",
	"AdminClientDeleted": "oauth2.administration.v1",
	"GroupCreated":       "iam.groups.v1", "GroupUpdated": "iam.groups.v1", "GroupDeleted": "iam.groups.v1",
	"GroupMemberAdded": "iam.groups.v1", "GroupMemberRemoved": "iam.groups.v1",
}

type OutboxEventSink struct{ Pool *pgxpool.Pool }

func (s *OutboxEventSink) Emit(ctx context.Context, event spec.DomainEvent) error {
	topic := eventTopics[event.EventType()]
	if topic == "" {
		return fmt.Errorf("no topic mapping for event %s", event.EventType())
	}
	payload, err := spec.MarshalDomainEvent(event)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO outbox(event_type,topic,payload) VALUES ($1,$2,$3)`,
		event.EventType(), topic, string(payload))
	return err
}
