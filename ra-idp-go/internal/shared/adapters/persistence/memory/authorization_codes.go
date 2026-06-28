package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// AuthorizationCodeStore (OAuth2)
// =====================================================================

type AuthorizationCodeStore struct {
	mu    sync.Mutex
	codes map[string]*spec.AuthorizationCodeRecord
}

func NewAuthorizationCodeStore() *AuthorizationCodeStore {
	return &AuthorizationCodeStore{codes: map[string]*spec.AuthorizationCodeRecord{}}
}

func (s *AuthorizationCodeStore) Save(_ context.Context, code *spec.AuthorizationCodeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&code.TenantID)
	s.codes[code.Code] = cloneAuthorizationCode(code)
	return nil
}

func (s *AuthorizationCodeStore) Find(_ context.Context, code string) (*spec.AuthorizationCodeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneAuthorizationCode(s.codes[code]), nil
}

func (s *AuthorizationCodeStore) Redeem(_ context.Context, code string, now time.Time) (*spec.AuthorizationCodeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.codes[code]
	if !ok {
		return nil, nil
	}
	if rec.State != spec.AuthCodeRecordIssued {
		return nil, nil
	}
	next, err := spec.TransitionAuthorizationCodeRecord(rec.State, spec.RecordEventRedeem)
	if err != nil {
		return nil, err
	}
	rec.State = next
	t := now.UTC()
	rec.RedeemedAt = &t
	return cloneAuthorizationCode(rec), nil
}

func (s *AuthorizationCodeStore) LinkFamily(_ context.Context, code, familyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.codes[code]
	if !ok {
		return errors.New("code not found")
	}
	id := familyID
	rec.IssuedFamilyID = &id
	return nil
}

func cloneAuthorizationCode(in *spec.AuthorizationCodeRecord) *spec.AuthorizationCodeRecord {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string(nil), in.Scopes...)
	return &out
}
