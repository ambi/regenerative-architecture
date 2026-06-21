package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// ClientRepository (OAuth2)
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
