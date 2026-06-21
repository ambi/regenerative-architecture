package memory

import (
	"context"
	"sync"
	"time"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// PARStore (OAuth2)
// =====================================================================

type PARStore struct {
	mu      sync.Mutex
	records map[string]*spec.PARRecord
}

func NewPARStore() *PARStore {
	return &PARStore{records: map[string]*spec.PARRecord{}}
}

func (s *PARStore) Save(_ context.Context, rec *spec.PARRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&rec.TenantID)
	s.records[rec.RequestURI] = rec
	return nil
}

func (s *PARStore) Find(_ context.Context, requestURI string) (*spec.PARRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records[requestURI], nil
}

func (s *PARStore) Consume(_ context.Context, requestURI string) (*spec.PARRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[requestURI]
	if !ok || rec.Used || time.Now().After(rec.ExpiresAt) {
		return nil, nil
	}
	rec.Used = true
	return rec, nil
}
