package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// ConsentRepository (OAuth2)
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
