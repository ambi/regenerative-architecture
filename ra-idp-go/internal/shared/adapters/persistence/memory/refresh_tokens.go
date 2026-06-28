package memory

import (
	"context"
	"errors"
	"sync"

	"ra-idp-go/internal/shared/spec"
)

// =====================================================================
// RefreshTokenStore (OAuth2, ファミリーローテーション対応)
// =====================================================================

type RefreshTokenStore struct {
	mu     sync.Mutex
	byHash map[string]*spec.RefreshTokenRecord
	byID   map[string]*spec.RefreshTokenRecord
}

func NewRefreshTokenStore() *RefreshTokenStore {
	return &RefreshTokenStore{
		byHash: map[string]*spec.RefreshTokenRecord{},
		byID:   map[string]*spec.RefreshTokenRecord{},
	}
}

func (s *RefreshTokenStore) FindByHash(_ context.Context, hash string) (*spec.RefreshTokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneRefreshToken(s.byHash[hash]), nil
}

func (s *RefreshTokenStore) Save(_ context.Context, rec *spec.RefreshTokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defaultTenant(&rec.TenantID)
	stored := cloneRefreshToken(rec)
	s.byHash[stored.Hash] = stored
	s.byID[stored.ID] = stored
	return nil
}

func (s *RefreshTokenStore) Rotate(_ context.Context, parentID string, newRec *spec.RefreshTokenRecord) (*spec.RefreshTokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parent, ok := s.byID[parentID]
	if !ok {
		return nil, errors.New("parent refresh token not found")
	}
	if parent.Rotated || parent.Revoked {
		return nil, nil
	}
	parent.Rotated = true
	defaultTenant(&newRec.TenantID)
	stored := cloneRefreshToken(newRec)
	s.byHash[stored.Hash] = stored
	s.byID[stored.ID] = stored
	return cloneRefreshToken(stored), nil
}

func (s *RefreshTokenStore) RevokeFamily(_ context.Context, familyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.byID {
		if rec.FamilyID == familyID {
			rec.Revoked = true
		}
	}
	return nil
}

func (s *RefreshTokenStore) DeleteAllForSub(_ context.Context, sub string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, rec := range s.byID {
		if rec.Sub == sub {
			delete(s.byID, id)
			delete(s.byHash, rec.Hash)
		}
	}
	return nil
}

func cloneRefreshToken(in *spec.RefreshTokenRecord) *spec.RefreshTokenRecord {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string(nil), in.Scopes...)
	if in.SenderConstraint != nil {
		senderConstraint := *in.SenderConstraint
		out.SenderConstraint = &senderConstraint
	}
	return &out
}
