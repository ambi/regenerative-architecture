package memory

// AuditEventStore は AuditEventRepository (SCL Trust component) の in-memory
// 実装。直近のオペレーション可視化を目的に、テナントごとに最大 maxEvents 件を
// FIFO で保持する。永続化はせず、本番では Postgres / SIEM 等に差し替える前提。

import (
	"context"
	"slices"
	"sync"

	"ra-idp-go/internal/oauth2/ports"
)

const (
	auditDefaultListLimit = 100
	auditMaxListLimit     = 1000
)

type AuditEventStore struct {
	mu        sync.RWMutex
	events    []*ports.AuditEventRecord
	byID      map[string]*ports.AuditEventRecord
	maxEvents int
}

// NewAuditEventStore は maxEvents を上限とするリングバッファ。0 を渡すと 10000 件を使う。
func NewAuditEventStore(maxEvents int) *AuditEventStore {
	if maxEvents <= 0 {
		maxEvents = 10000
	}
	return &AuditEventStore{
		events:    make([]*ports.AuditEventRecord, 0, 1024),
		byID:      map[string]*ports.AuditEventRecord{},
		maxEvents: maxEvents,
	}
}

func (s *AuditEventStore) Append(_ context.Context, rec *ports.AuditEventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.ID == "" || rec.Type == "" {
		return nil
	}
	if _, exists := s.byID[rec.ID]; exists {
		return nil
	}
	s.events = append(s.events, rec)
	s.byID[rec.ID] = rec
	// 上限超過時は古い方から落とす。byID も同期。
	if overflow := len(s.events) - s.maxEvents; overflow > 0 {
		for _, dropped := range s.events[:overflow] {
			delete(s.byID, dropped.ID)
		}
		s.events = append(s.events[:0], s.events[overflow:]...)
	}
	return nil
}

func (s *AuditEventStore) List(_ context.Context, q ports.AuditEventQuery) ([]*ports.AuditEventRecord, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = auditDefaultListLimit
	}
	if limit > auditMaxListLimit {
		limit = auditMaxListLimit
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	// OccurredAt 降順 (新しい順) で limit 件まで集める。
	result := make([]*ports.AuditEventRecord, 0, auditMaxListLimit)
	for i := len(s.events) - 1; i >= 0; i-- {
		rec := s.events[i]
		if !auditEventMatches(rec, q) {
			continue
		}
		result = append(result, rec)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *AuditEventStore) FindByID(_ context.Context, id string) (*ports.AuditEventRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byID[id], nil
}

// DeleteOlderThan は ADR-045 の保持期間 sweep。type ごとに cutoff より古い行を物理削除し、
// 削除件数を返す。Keep に挙げた type は削除しない。idempotent で、複数回呼んでも収束する。
func (s *AuditEventStore) DeleteOlderThan(_ context.Context, cutoff ports.RetentionCutoff) (int64, error) {
	keep := make(map[string]bool, len(cutoff.Keep))
	for _, t := range cutoff.Keep {
		keep[t] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted int64
	kept := s.events[:0]
	for _, rec := range s.events {
		before, ok := cutoff.ByType[rec.Type]
		if !ok {
			before = cutoff.Default
		}
		if !keep[rec.Type] && !before.IsZero() && rec.OccurredAt.Before(before) {
			delete(s.byID, rec.ID)
			deleted++
			continue
		}
		kept = append(kept, rec)
	}
	s.events = kept
	return deleted, nil
}

func auditEventMatches(rec *ports.AuditEventRecord, q ports.AuditEventQuery) bool {
	if !q.AllTenants && q.TenantID != "" && rec.TenantID != q.TenantID {
		return false
	}
	if q.Type != "" && rec.Type != q.Type {
		return false
	}
	if len(q.Types) > 0 && !slices.Contains(q.Types, rec.Type) {
		return false
	}
	if q.Sub != "" {
		sub, _ := rec.Payload["sub"].(string)
		if sub != q.Sub {
			return false
		}
	}
	for key, want := range q.PayloadEquals {
		if got, _ := rec.Payload[key].(string); got != want {
			return false
		}
	}
	if !q.After.IsZero() && rec.OccurredAt.Before(q.After) {
		return false
	}
	if !q.Before.IsZero() && rec.OccurredAt.After(q.Before) {
		return false
	}
	return true
}
