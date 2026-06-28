package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/shared/spec"
)

// SamlServiceProviderRepository は SAML 2.0 SP trust を PostgreSQL に永続化する。
// URI 識別子と claim policy は tenant scope の行と JSONB に閉じる。
type SamlServiceProviderRepository struct{ Pool *pgxpool.Pool }

const samlServiceProviderSelect = `SELECT tenant_id,entity_id,display_name,acs_urls,slo_url,audience,
claim_policy,sign_assertion,sign_response,want_authn_requests_signed,authn_request_signing_certificate_pem,
created_at,updated_at FROM saml_service_providers`

func scanSamlServiceProvider(row rowScanner) (*spec.SamlServiceProvider, error) {
	var (
		sp          spec.SamlServiceProvider
		acsURLs     []byte
		claimPolicy []byte
	)
	err := row.Scan(&sp.TenantID, &sp.EntityID, &sp.DisplayName, &acsURLs, &sp.SLOURL, &sp.Audience,
		&claimPolicy, &sp.SignAssertion, &sp.SignResponse, &sp.WantAuthnRequestsSigned,
		&sp.AuthnRequestSigningCertificatePEM, &sp.CreatedAt, &sp.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(acsURLs) > 0 {
		if err := json.Unmarshal(acsURLs, &sp.ACSURLs); err != nil {
			return nil, err
		}
	}
	if len(claimPolicy) > 0 {
		if err := json.Unmarshal(claimPolicy, &sp.ClaimPolicy); err != nil {
			return nil, err
		}
	}
	return &sp, nil
}

func (r *SamlServiceProviderRepository) FindByEntityID(ctx context.Context, tenantID, entityID string) (*spec.SamlServiceProvider, error) {
	return scanSamlServiceProvider(r.Pool.QueryRow(ctx,
		samlServiceProviderSelect+" WHERE tenant_id=$1 AND entity_id=$2", tenantID, entityID))
}

func (r *SamlServiceProviderRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.SamlServiceProvider, error) {
	rows, err := r.Pool.Query(ctx, samlServiceProviderSelect+" WHERE tenant_id=$1 ORDER BY entity_id", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.SamlServiceProvider{}
	for rows.Next() {
		sp, err := scanSamlServiceProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (r *SamlServiceProviderRepository) Save(ctx context.Context, sp *spec.SamlServiceProvider) error {
	acsURLs := sp.ACSURLs
	if acsURLs == nil {
		acsURLs = []string{}
	}
	encodedACSURLs, err := json.Marshal(acsURLs)
	if err != nil {
		return err
	}
	encodedClaimPolicy, err := json.Marshal(sp.ClaimPolicy)
	if err != nil {
		return err
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO saml_service_providers (
 tenant_id,entity_id,display_name,acs_urls,slo_url,audience,claim_policy,sign_assertion,sign_response,
 want_authn_requests_signed,authn_request_signing_certificate_pem,created_at,updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (tenant_id,entity_id) DO UPDATE SET display_name=EXCLUDED.display_name,
 acs_urls=EXCLUDED.acs_urls,slo_url=EXCLUDED.slo_url,audience=EXCLUDED.audience,
 claim_policy=EXCLUDED.claim_policy,sign_assertion=EXCLUDED.sign_assertion,
 sign_response=EXCLUDED.sign_response,want_authn_requests_signed=EXCLUDED.want_authn_requests_signed,
 authn_request_signing_certificate_pem=EXCLUDED.authn_request_signing_certificate_pem,
 updated_at=EXCLUDED.updated_at`,
		sp.TenantID, sp.EntityID, sp.DisplayName, encodedACSURLs, sp.SLOURL, sp.Audience,
		encodedClaimPolicy, sp.SignAssertion, sp.SignResponse, sp.WantAuthnRequestsSigned,
		sp.AuthnRequestSigningCertificatePEM, sp.CreatedAt, sp.UpdatedAt)
	return err
}

func (r *SamlServiceProviderRepository) Delete(ctx context.Context, tenantID, entityID string) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM saml_service_providers WHERE tenant_id=$1 AND entity_id=$2", tenantID, entityID)
	return err
}
