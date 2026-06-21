package memory

import (
	"context"
	"sync"
	"time"
)

// =====================================================================
// Replay stores (OAuth2: DPoP / client assertion jti)
// =====================================================================

type replayEntry struct {
	expiresAt time.Time
}

type replayStore struct {
	mu   sync.Mutex
	seen map[string]replayEntry
}

func newReplayStore() *replayStore { return &replayStore{seen: map[string]replayEntry{}} }

func (s *replayStore) recordIfNew(jti string, windowSeconds int, now time.Time) (bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.seen {
		if !now.Before(e.expiresAt) {
			delete(s.seen, k)
		}
	}
	if _, ok := s.seen[jti]; ok {
		return false, nil
	}
	s.seen[jti] = replayEntry{expiresAt: now.Add(time.Duration(windowSeconds) * time.Second)}
	return true, nil
}

type DpopReplayStore struct{ inner *replayStore }

func NewDpopReplayStore() *DpopReplayStore { return &DpopReplayStore{inner: newReplayStore()} }

func (s *DpopReplayStore) RecordIfNew(_ context.Context, jti string, windowSeconds int, now time.Time) (bool, error) {
	return s.inner.recordIfNew(jti, windowSeconds, now)
}

type ClientAssertionReplayStore struct{ inner *replayStore }

func NewClientAssertionReplayStore() *ClientAssertionReplayStore {
	return &ClientAssertionReplayStore{inner: newReplayStore()}
}

func (s *ClientAssertionReplayStore) RecordIfNew(_ context.Context, jti string, windowSeconds int, now time.Time) (bool, error) {
	return s.inner.recordIfNew(jti, windowSeconds, now)
}
