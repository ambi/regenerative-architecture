// Package memory: 永続化アダプタの in-memory 実装（デモ・テスト用）。
// TS adapters/persistence/memory/*.ts に対応。
package memory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"ra-idp-go/internal/spec"
)

// =====================================================================
// ClientRepository
// =====================================================================

type ClientRepository struct {
	mu      sync.RWMutex
	clients map[string]*spec.Client
}

func NewClientRepository() *ClientRepository {
	return &ClientRepository{clients: map[string]*spec.Client{}}
}

func (r *ClientRepository) Seed(c *spec.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c.ClientID] = c
}

func (r *ClientRepository) FindByID(_ context.Context, clientID string) (*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[clientID], nil
}

func (r *ClientRepository) Save(_ context.Context, c *spec.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c.ClientID] = c
	return nil
}

func (r *ClientRepository) Delete(_ context.Context, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, clientID)
	return nil
}

func (r *ClientRepository) FindAll(_ context.Context) ([]*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Client, 0, len(r.clients))
	for _, c := range r.clients {
		out = append(out, c)
	}
	return out, nil
}

// =====================================================================
// UserRepository
// =====================================================================

type UserRepository struct {
	mu     sync.RWMutex
	bySub  map[string]*spec.User
	byUser map[string]*spec.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{bySub: map[string]*spec.User{}, byUser: map[string]*spec.User{}}
}

func (r *UserRepository) Seed(u *spec.User) {
	_ = r.Save(context.Background(), u)
}

func (r *UserRepository) Save(_ context.Context, u *spec.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bySub[u.Sub] = u
	r.byUser[u.PreferredUsername] = u
	return nil
}

func (r *UserRepository) FindBySub(_ context.Context, sub string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bySub[sub], nil
}

func (r *UserRepository) FindByUsername(_ context.Context, username string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byUser[username], nil
}

// =====================================================================
// ConsentRepository
// =====================================================================

type ConsentRepository struct {
	mu       sync.RWMutex
	consents map[string]*spec.Consent
}

func NewConsentRepository() *ConsentRepository {
	return &ConsentRepository{consents: map[string]*spec.Consent{}}
}

func (r *ConsentRepository) Find(_ context.Context, sub, clientID string) (*spec.Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.consents[consentKey(sub, clientID)], nil
}

func (r *ConsentRepository) Save(_ context.Context, c *spec.Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.consents[consentKey(c.Sub, c.ClientID)] = c
	return nil
}

func (r *ConsentRepository) Revoke(_ context.Context, sub, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.consents[consentKey(sub, clientID)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	c.State = spec.ConsentRevoked
	c.RevokedAt = &now
	return nil
}

func consentKey(sub, clientID string) string { return sub + "|" + clientID }

// =====================================================================
// AuthorizationRequestStore
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

// =====================================================================
// AuthorizationCodeStore
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

// =====================================================================
// PARStore
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

// =====================================================================
// RefreshTokenStore (ファミリーローテーション対応)
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

// =====================================================================
// DeviceCodeStore
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

func cloneAuthorizationCode(in *spec.AuthorizationCodeRecord) *spec.AuthorizationCodeRecord {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string(nil), in.Scopes...)
	return &out
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

func cloneDeviceAuthorization(in *spec.DeviceAuthorization) *spec.DeviceAuthorization {
	if in == nil {
		return nil
	}
	out := *in
	out.Scopes = append([]string(nil), in.Scopes...)
	return &out
}

// =====================================================================
// Replay stores
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

type AccessTokenDenylist struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func NewAccessTokenDenylist() *AccessTokenDenylist {
	return &AccessTokenDenylist{entries: map[string]time.Time{}}
}

func (d *AccessTokenDenylist) Add(_ context.Context, jti string, expiresAt time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[jti] = expiresAt
	return nil
}

func (d *AccessTokenDenylist) IsRevoked(_ context.Context, jti string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	expiresAt, ok := d.entries[jti]
	if ok && time.Now().After(expiresAt) {
		delete(d.entries, jti)
		return false, nil
	}
	return ok, nil
}

// =====================================================================
// SessionStore
// =====================================================================

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*spec.LoginSession
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: map[string]*spec.LoginSession{}}
}

func (s *SessionStore) Save(_ context.Context, sess *spec.LoginSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	return nil
}

func (s *SessionStore) Find(_ context.Context, id string) (*spec.LoginSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, nil
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, id)
		return nil, nil
	}
	return sess, nil
}

func (s *SessionStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	return nil
}
