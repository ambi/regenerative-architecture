package memory

import (
	"context"
	"sync"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

// =====================================================================
// EmailChangeTokenStore (Authentication)
// =====================================================================

type EmailChangeTokenStore struct {
	mu      sync.Mutex
	records map[string]authports.EmailChangeTokenRecord
}

func NewEmailChangeTokenStore() *EmailChangeTokenStore {
	return &EmailChangeTokenStore{records: map[string]authports.EmailChangeTokenRecord{}}
}

func (s *EmailChangeTokenStore) Save(
	_ context.Context,
	record authports.EmailChangeTokenRecord,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, existing := range s.records {
		if existing.Sub == record.Sub {
			delete(s.records, hash)
		}
	}
	s.records[record.TokenHash] = record
	return nil
}

func (s *EmailChangeTokenStore) Consume(
	_ context.Context,
	tokenHash string,
	now time.Time,
) (*authports.EmailChangeTokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[tokenHash]
	if !ok {
		return nil, nil
	}
	delete(s.records, tokenHash)
	if !now.Before(record.ExpiresAt) {
		return nil, nil
	}
	return &record, nil
}
