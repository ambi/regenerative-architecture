package memory

import (
	"context"
	"sync"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// DeviceCodeStore (OAuth2)
// =====================================================================

type DeviceCodeStore struct {
	mu         sync.Mutex
	byHash     map[string]*spec.DeviceAuthorization
	byUserCode map[string]*spec.DeviceAuthorization
}

func NewDeviceCodeStore() *DeviceCodeStore {
	return &DeviceCodeStore{
		byHash:     map[string]*spec.DeviceAuthorization{},
		byUserCode: map[string]*spec.DeviceAuthorization{},
	}
}

func (s *DeviceCodeStore) Save(_ context.Context, rec *spec.DeviceAuthorization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&rec.TenantID)
	stored := cloneDeviceAuthorization(rec)
	s.byHash[stored.DeviceCodeHash] = stored
	s.byUserCode[stored.UserCode] = stored
	return nil
}

func (s *DeviceCodeStore) FindByDeviceCodeHash(_ context.Context, hash string) (*spec.DeviceAuthorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneDeviceAuthorization(s.byHash[hash]), nil
}

func (s *DeviceCodeStore) FindByUserCode(_ context.Context, userCode string) (*spec.DeviceAuthorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneDeviceAuthorization(s.byUserCode[userCode]), nil
}

func (s *DeviceCodeStore) Update(_ context.Context, rec *spec.DeviceAuthorization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&rec.TenantID)
	stored := cloneDeviceAuthorization(rec)
	s.byHash[stored.DeviceCodeHash] = stored
	s.byUserCode[stored.UserCode] = stored
	return nil
}

func (s *DeviceCodeStore) Exchange(_ context.Context, deviceCodeHash string) (*spec.DeviceAuthorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.byHash[deviceCodeHash]
	if rec == nil || rec.State != spec.DeviceFlowApproved {
		return nil, nil
	}
	next, err := spec.TransitionDeviceCodeFlow(rec.State, spec.DeviceEventExchange)
	if err != nil {
		return nil, err
	}
	rec.State = next
	return cloneDeviceAuthorization(rec), nil
}

func (s *DeviceCodeStore) DeleteAllForSub(_ context.Context, sub string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, rec := range s.byHash {
		if rec.Sub != nil && *rec.Sub == sub {
			delete(s.byHash, hash)
			delete(s.byUserCode, rec.UserCode)
		}
	}
	return nil
}

func cloneDeviceAuthorization(in *spec.DeviceAuthorization) *spec.DeviceAuthorization {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string(nil), in.Scopes...)
	return &out
}
