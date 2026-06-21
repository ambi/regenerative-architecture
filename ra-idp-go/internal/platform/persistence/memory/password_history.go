package memory

import (
	"context"
	"sync"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

// =====================================================================
// PasswordHistoryRepository (Authentication)
// =====================================================================

type PasswordHistoryRepository struct {
	mu      sync.RWMutex
	entries map[string][]authports.PasswordHistoryEntry
}

func NewPasswordHistoryRepository() *PasswordHistoryRepository {
	return &PasswordHistoryRepository{entries: map[string][]authports.PasswordHistoryEntry{}}
}

func (r *PasswordHistoryRepository) Recent(
	_ context.Context,
	sub string,
	depth int,
) ([]authports.PasswordHistoryEntry, error) {
	if depth <= 0 {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	history := r.entries[sub]
	if len(history) == 0 {
		return nil, nil
	}
	if depth > len(history) {
		depth = len(history)
	}
	out := make([]authports.PasswordHistoryEntry, depth)
	copy(out, history[:depth])
	return out, nil
}

func (r *PasswordHistoryRepository) Add(
	_ context.Context,
	sub, encoded string,
	now time.Time,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[sub] = append([]authports.PasswordHistoryEntry{{
		Encoded:   encoded,
		CreatedAt: now,
	}}, r.entries[sub]...)
	return nil
}

func (r *PasswordHistoryRepository) DeleteAllForSub(_ context.Context, sub string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, sub)
	return nil
}
