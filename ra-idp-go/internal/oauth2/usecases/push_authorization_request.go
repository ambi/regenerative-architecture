// /par (RFC 9126 Pushed Authorization Request)
package usecases

import (
	"context"
	"time"

	"ra-idp-go/internal/oauth2/ports"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

type PARInput struct {
	ClientID   string
	Parameters map[string]string
}

type PARResult struct {
	RequestURI string
	ExpiresIn  int
}

type PARDeps struct {
	ClientRepo          ports.OAuth2ClientRepository
	Store               ports.PARStore
	AuthzDetailTypeRepo ports.AuthorizationDetailTypeRepository
	Emit                func(spec.DomainEvent)
}

func PushAuthorizationRequest(ctx context.Context, deps PARDeps, in PARInput, now time.Time) (*PARResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, NewOAuthError("invalid_client", "未知の client_id")
	}
	// RFC 9396 — authorization_details があれば push 時点で fail-closed 検証する (ADR-050)。
	if raw := in.Parameters["authorization_details"]; raw != "" {
		details, err := ParseAuthorizationDetails(raw)
		if err != nil {
			return nil, err
		}
		if err := ValidateAuthorizationDetails(ctx, deps.AuthzDetailTypeRepo, details); err != nil {
			return nil, err
		}
	}
	id, err := generateOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	requestURI := "urn:ietf:params:oauth:request_uri:" + id
	rec := &spec.PARRecord{
		TenantID:   tenantID,
		RequestURI: requestURI,
		ClientID:   in.ClientID,
		Parameters: in.Parameters,
		IssuedAt:   now,
		ExpiresAt:  now.Add(90 * time.Second), // RFC 9126 §4 推奨上限
	}
	if err := rec.Validate(); err != nil {
		return nil, err
	}
	if err := deps.Store.Save(ctx, rec); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.PARStored{At: now, TenantID: tenantID, RequestURI: requestURI, ClientID: in.ClientID})
	return &PARResult{RequestURI: requestURI, ExpiresIn: 90}, nil
}
