package memory

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// AuthorizationRequestStore (OAuth2)
// =====================================================================

type AuthorizationRequestStore struct {
	mu       sync.RWMutex
	requests map[string]*spec.AuthorizationRequest
}

func NewAuthorizationRequestStore() *AuthorizationRequestStore {
	return &AuthorizationRequestStore{requests: map[string]*spec.AuthorizationRequest{}}
}

func (s *AuthorizationRequestStore) Save(_ context.Context, req *spec.AuthorizationRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&req.TenantID)
	s.requests[req.ID] = req
	return nil
}

func (s *AuthorizationRequestStore) Find(_ context.Context, id string) (*spec.AuthorizationRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.requests[id], nil
}

func (s *AuthorizationRequestStore) UpdateState(_ context.Context, id string, state spec.AuthorizationCodeFlowState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("authorization request %q not found", id)
	}
	next, err := spec.TransitionAuthorizationCodeFlow(req.State, eventForTargetState(req.State, state))
	if err != nil {
		return fmt.Errorf("invalid transition %q → %q: %w", req.State, state, err)
	}
	req.State = next
	return nil
}

func (s *AuthorizationRequestStore) AttachAuthentication(
	_ context.Context,
	id, sub string,
	authTime int64,
	amr []string,
	acr string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("authorization request %q not found", id)
	}
	req.Sub = &sub
	req.AuthTime = &authTime
	req.AMR = slices.Clone(amr)
	req.ACR = &acr
	return nil
}

func eventForTargetState(_, to spec.AuthorizationCodeFlowState) spec.AuthorizationCodeFlowEvent {
	switch to {
	case spec.AuthFlowAuthenticationPending:
		return spec.EventStartAuthentication
	case spec.AuthFlowAuthenticated:
		return spec.EventAuthenticateUser
	case spec.AuthFlowConsentPending:
		return spec.EventRequestConsent
	case spec.AuthFlowConsented:
		return spec.EventGrantConsent
	case spec.AuthFlowCodeIssued:
		return spec.EventIssueCode
	case spec.AuthFlowExchanged:
		return spec.EventRedeemCode
	case spec.AuthFlowRejected:
		return spec.EventRejectAuthorization
	case spec.AuthFlowExpired:
		return spec.EventExpireRequest
	}
	return spec.AuthorizationCodeFlowEvent("unknown")
}
