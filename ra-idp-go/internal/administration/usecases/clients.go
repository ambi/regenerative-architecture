package usecases

import (
	"context"
	"errors"
	"slices"
	"time"

	oauthports "ra-idp-go/internal/oauth2/ports"
	oauthusecases "ra-idp-go/internal/oauth2/usecases"
	"ra-idp-go/internal/spec"
	"ra-idp-go/internal/tenancy"
)

var ErrClientNotFound = errors.New("client not found")

type ClientDeps struct {
	ClientRepo oauthports.ClientRepository
	Emit       func(spec.DomainEvent)
}

type CreateClientInput struct {
	ActorSub     string
	Registration oauthusecases.RegisterClientInput
	Now          time.Time
}

func CreateClient(
	ctx context.Context,
	deps ClientDeps,
	in CreateClientInput,
) (*oauthusecases.RegisterClientResult, error) {
	result, err := oauthusecases.RegisterClient(ctx, oauthusecases.RegisterClientDeps{
		ClientRepo: deps.ClientRepo,
	}, in.Registration, in.Now)
	if err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.AdminClientCreated{
		At: normalizedNow(in.Now), ActorSub: in.ActorSub, ClientID: result.Client.ClientID,
	})
	return result, nil
}

type UpdateClientInput struct {
	ActorSub        string
	ClientID        string
	ClientName      *string
	RedirectURIs    *[]string
	GrantTypes      *[]spec.GrantType
	ResponseTypes   *[]spec.ResponseType
	Scope           *string
	RequirePAR      *bool
	DpopBoundTokens *bool
	Now             time.Time
}

func UpdateClient(ctx context.Context, deps ClientDeps, in UpdateClientInput) (*spec.Client, error) {
	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, in.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, ErrClientNotFound
	}
	updated := *client
	changed := []string{}
	if in.ClientName != nil && !equalOptionalString(client.ClientName, in.ClientName) {
		updated.ClientName = in.ClientName
		changed = append(changed, "client_name")
	}
	if in.RedirectURIs != nil && !slices.Equal(client.RedirectURIs, *in.RedirectURIs) {
		updated.RedirectURIs = slices.Clone(*in.RedirectURIs)
		changed = append(changed, "redirect_uris")
	}
	if in.GrantTypes != nil && !slices.Equal(client.GrantTypes, *in.GrantTypes) {
		updated.GrantTypes = slices.Clone(*in.GrantTypes)
		changed = append(changed, "grant_types")
	}
	if in.ResponseTypes != nil && !slices.Equal(client.ResponseTypes, *in.ResponseTypes) {
		updated.ResponseTypes = slices.Clone(*in.ResponseTypes)
		changed = append(changed, "response_types")
	}
	if in.Scope != nil && client.Scope != *in.Scope {
		updated.Scope = *in.Scope
		changed = append(changed, "scope")
	}
	if in.RequirePAR != nil && client.RequirePushedAuthorizationRequests != *in.RequirePAR {
		updated.RequirePushedAuthorizationRequests = *in.RequirePAR
		changed = append(changed, "require_pushed_authorization_requests")
	}
	if in.DpopBoundTokens != nil && client.DpopBoundAccessTokens != *in.DpopBoundTokens {
		updated.DpopBoundAccessTokens = *in.DpopBoundTokens
		changed = append(changed, "dpop_bound_access_tokens")
	}
	if len(changed) == 0 {
		return &updated, nil
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	if err := deps.ClientRepo.Save(ctx, &updated); err != nil {
		return nil, err
	}
	emit(deps.Emit, &spec.AdminClientUpdated{
		At: normalizedNow(in.Now), ActorSub: in.ActorSub, ClientID: client.ClientID,
		ChangedFields: changed,
	})
	return &updated, nil
}

func DeleteClient(
	ctx context.Context,
	deps ClientDeps,
	actorSub, clientID string,
	now time.Time,
) error {
	tenantID := tenancy.TenantID(ctx)
	client, err := deps.ClientRepo.FindByID(ctx, tenantID, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return ErrClientNotFound
	}
	if err := deps.ClientRepo.Delete(ctx, tenantID, clientID); err != nil {
		return err
	}
	emit(deps.Emit, &spec.AdminClientDeleted{
		At: normalizedNow(now), ActorSub: actorSub, ClientID: clientID,
	})
	return nil
}
