package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ra-idp-go/internal/spec"
)

// WsFedRelyingPartyRepository は WS-Federation RP trust を PostgreSQL に永続化する。
// wtrealm は URI として扱い、tenant scope の主キーに含める。
type WsFedRelyingPartyRepository struct{ Pool *pgxpool.Pool }

const wsFedRelyingPartySelect = `SELECT tenant_id,wtrealm,display_name,reply_urls,audience,token_type,
claim_policy,entra_profile,created_at,updated_at FROM wsfed_relying_parties`

func scanWsFedRelyingParty(row rowScanner) (*spec.WsFedRelyingParty, error) {
	var (
		rp           spec.WsFedRelyingParty
		replyURLs    []byte
		claimPolicy  []byte
		entraProfile []byte
		tokenType    string
	)
	err := row.Scan(&rp.TenantID, &rp.Wtrealm, &rp.DisplayName, &replyURLs, &rp.Audience, &tokenType,
		&claimPolicy, &entraProfile, &rp.CreatedAt, &rp.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rp.TokenType = spec.WsFedTokenType(tokenType)
	if len(replyURLs) > 0 {
		if err := json.Unmarshal(replyURLs, &rp.ReplyURLs); err != nil {
			return nil, err
		}
	}
	if len(claimPolicy) > 0 {
		if err := json.Unmarshal(claimPolicy, &rp.ClaimPolicy); err != nil {
			return nil, err
		}
	}
	if len(entraProfile) > 0 {
		var profile spec.EntraFederationProfile
		if err := json.Unmarshal(entraProfile, &profile); err != nil {
			return nil, err
		}
		rp.EntraProfile = &profile
	}
	return &rp, nil
}

func (r *WsFedRelyingPartyRepository) FindByWtrealm(ctx context.Context, tenantID, wtrealm string) (*spec.WsFedRelyingParty, error) {
	return scanWsFedRelyingParty(r.Pool.QueryRow(ctx,
		wsFedRelyingPartySelect+" WHERE tenant_id=$1 AND wtrealm=$2", tenantID, wtrealm))
}

func (r *WsFedRelyingPartyRepository) ListByTenant(ctx context.Context, tenantID string) ([]*spec.WsFedRelyingParty, error) {
	rows, err := r.Pool.Query(ctx, wsFedRelyingPartySelect+" WHERE tenant_id=$1 ORDER BY wtrealm", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*spec.WsFedRelyingParty{}
	for rows.Next() {
		rp, err := scanWsFedRelyingParty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rp)
	}
	return out, rows.Err()
}

func (r *WsFedRelyingPartyRepository) Save(ctx context.Context, rp *spec.WsFedRelyingParty) error {
	replyURLs := rp.ReplyURLs
	if replyURLs == nil {
		replyURLs = []string{}
	}
	encodedReplyURLs, err := json.Marshal(replyURLs)
	if err != nil {
		return err
	}
	encodedClaimPolicy, err := json.Marshal(rp.ClaimPolicy)
	if err != nil {
		return err
	}
	var encodedEntraProfile []byte
	if rp.EntraProfile != nil {
		encodedEntraProfile, err = json.Marshal(rp.EntraProfile)
		if err != nil {
			return err
		}
	}
	_, err = r.Pool.Exec(ctx, `
INSERT INTO wsfed_relying_parties (
 tenant_id,wtrealm,display_name,reply_urls,audience,token_type,claim_policy,entra_profile,created_at,updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (tenant_id,wtrealm) DO UPDATE SET display_name=EXCLUDED.display_name,
 reply_urls=EXCLUDED.reply_urls,audience=EXCLUDED.audience,token_type=EXCLUDED.token_type,
 claim_policy=EXCLUDED.claim_policy,entra_profile=EXCLUDED.entra_profile,updated_at=EXCLUDED.updated_at`,
		rp.TenantID, rp.Wtrealm, rp.DisplayName, encodedReplyURLs, rp.Audience, string(rp.TokenType),
		encodedClaimPolicy, encodedEntraProfile, rp.CreatedAt, rp.UpdatedAt)
	return err
}

func (r *WsFedRelyingPartyRepository) Delete(ctx context.Context, tenantID, wtrealm string) error {
	_, err := r.Pool.Exec(ctx,
		"DELETE FROM wsfed_relying_parties WHERE tenant_id=$1 AND wtrealm=$2", tenantID, wtrealm)
	return err
}
