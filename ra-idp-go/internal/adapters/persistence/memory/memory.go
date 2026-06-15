// Package memory: 永続化アダプタの in-memory 実装（デモ・テスト用）。
// TS adapters/persistence/memory/*.ts に対応。
package memory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
	"ra-idp-go/internal/spec"
)

// =====================================================================
// TenantRepository
// =====================================================================

type TenantRepository struct {
	mu      sync.RWMutex
	tenants map[string]*spec.Tenant
}

func NewTenantRepository() *TenantRepository {
	return &TenantRepository{tenants: map[string]*spec.Tenant{}}
}

func (r *TenantRepository) FindByID(_ context.Context, id string) (*spec.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if tenant := r.tenants[id]; tenant != nil {
		cloned := *tenant
		return &cloned, nil
	}
	return nil, nil
}

func (r *TenantRepository) FindAll(_ context.Context) ([]*spec.Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		cloned := *tenant
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.Tenant) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func (r *TenantRepository) Save(_ context.Context, tenant *spec.Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *tenant
	r.tenants[tenant.ID] = &cloned
	return nil
}

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
	_ = r.Save(context.Background(), c)
}

func (r *ClientRepository) FindByID(_ context.Context, tenantID, clientID string) (*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[tenantKey(tenantID, clientID)], nil
}

func (r *ClientRepository) Save(_ context.Context, c *spec.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defaultTenant(&c.TenantID)
	r.clients[tenantKey(c.TenantID, c.ClientID)] = c
	return nil
}

func (r *ClientRepository) Delete(_ context.Context, tenantID, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, tenantKey(tenantID, clientID))
	return nil
}

func (r *ClientRepository) FindAll(_ context.Context, tenantID string) ([]*spec.Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Client, 0, len(r.clients))
	for _, c := range r.clients {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
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
	if existing := r.bySub[u.Sub]; existing != nil &&
		existing.PreferredUsername != u.PreferredUsername {
		delete(r.byUser, tenantKey(existing.TenantID, existing.PreferredUsername))
	}
	defaultTenant(&u.TenantID)
	r.bySub[u.Sub] = u
	r.byUser[tenantKey(u.TenantID, u.PreferredUsername)] = u
	return nil
}

func (r *UserRepository) FindBySub(_ context.Context, sub string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user := r.bySub[sub]
	if user == nil || user.DeletedAt != nil {
		return nil, nil
	}
	return user, nil
}

func (r *UserRepository) FindBySubIncludingDeleted(_ context.Context, sub string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bySub[sub], nil
}

func (r *UserRepository) FindByUsername(_ context.Context, tenantID, username string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user := r.byUser[tenantKey(tenantID, username)]
	if user == nil || user.DeletedAt != nil {
		return nil, nil
	}
	return user, nil
}

func (r *UserRepository) FindByEmail(_ context.Context, tenantID, email string) (*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.bySub {
		if user.DeletedAt != nil {
			continue
		}
		if user.TenantID == tenantID && user.Email != nil && strings.EqualFold(*user.Email, email) {
			return user, nil
		}
	}
	return nil, nil
}

func (r *UserRepository) FindAll(_ context.Context, tenantID string) ([]*spec.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.User, 0, len(r.bySub))
	for _, user := range r.bySub {
		if user.TenantID == tenantID && user.DeletedAt == nil {
			out = append(out, user)
		}
	}
	slices.SortFunc(out, func(a, b *spec.User) int {
		return strings.Compare(a.PreferredUsername, b.PreferredUsername)
	})
	return out, nil
}

// =====================================================================
// PasswordHistoryRepository
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

// =====================================================================
// PasswordResetTokenStore
// =====================================================================

type PasswordResetTokenStore struct {
	mu      sync.Mutex
	records map[string]authports.PasswordResetTokenRecord
}

func NewPasswordResetTokenStore() *PasswordResetTokenStore {
	return &PasswordResetTokenStore{records: map[string]authports.PasswordResetTokenRecord{}}
}

func (s *PasswordResetTokenStore) Save(
	_ context.Context,
	record authports.PasswordResetTokenRecord,
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

func (s *PasswordResetTokenStore) Consume(
	_ context.Context,
	tokenHash string,
	now time.Time,
) (*authports.PasswordResetTokenRecord, error) {
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

func (r *ConsentRepository) Find(_ context.Context, tenantID, sub, clientID string) (*spec.Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	consent := r.consents[consentKey(tenantID, sub, clientID)]
	if consent == nil {
		return nil, nil
	}
	cloned := *consent
	cloned.Scopes = slices.Clone(consent.Scopes)
	return &cloned, nil
}

func (r *ConsentRepository) FindAll(_ context.Context, tenantID string) ([]*spec.Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*spec.Consent, 0)
	for _, consent := range r.consents {
		if consent.TenantID != tenantID {
			continue
		}
		cloned := *consent
		cloned.Scopes = slices.Clone(consent.Scopes)
		out = append(out, &cloned)
	}
	slices.SortFunc(out, func(a, b *spec.Consent) int {
		if a.Sub != b.Sub {
			return strings.Compare(a.Sub, b.Sub)
		}
		return strings.Compare(a.ClientID, b.ClientID)
	})
	return out, nil
}

func (r *ConsentRepository) Save(_ context.Context, c *spec.Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *c
	defaultTenant(&cloned.TenantID)
	cloned.Scopes = slices.Clone(c.Scopes)
	r.consents[consentKey(cloned.TenantID, cloned.Sub, cloned.ClientID)] = &cloned
	return nil
}

func (r *ConsentRepository) Revoke(_ context.Context, tenantID, sub, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.consents[consentKey(tenantID, sub, clientID)]
	if !ok {
		return nil
	}
	now := time.Now().UTC()
	c.State = spec.ConsentRevoked
	c.RevokedAt = &now
	return nil
}

func (r *ConsentRepository) DeleteAllForSub(_ context.Context, sub string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, consent := range r.consents {
		if consent.Sub == sub {
			delete(r.consents, key)
		}
	}
	return nil
}

func consentKey(tenantID, sub, clientID string) string {
	return tenantKey(tenantID, sub+"|"+clientID)
}

// =====================================================================
// MfaFactorRepository
// =====================================================================

type MfaFactorRepository struct {
	mu      sync.RWMutex
	factors map[string]*spec.MfaFactor
}

func NewMfaFactorRepository() *MfaFactorRepository {
	return &MfaFactorRepository{factors: map[string]*spec.MfaFactor{}}
}

func (r *MfaFactorRepository) ListBySub(_ context.Context, sub string) ([]*spec.MfaFactor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []*spec.MfaFactor{}
	for _, factor := range r.factors {
		if factor.Sub == sub {
			out = append(out, cloneMfaFactor(factor))
		}
	}
	return out, nil
}

func (r *MfaFactorRepository) Find(
	_ context.Context,
	sub string,
	factorType spec.MfaFactorType,
) (*spec.MfaFactor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneMfaFactor(r.factors[mfaFactorKey(sub, factorType)]), nil
}

func (r *MfaFactorRepository) Save(_ context.Context, factor *spec.MfaFactor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factors[mfaFactorKey(factor.Sub, factor.Type)] = cloneMfaFactor(factor)
	return nil
}

func (r *MfaFactorRepository) Delete(_ context.Context, sub string, factorType spec.MfaFactorType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.factors, mfaFactorKey(sub, factorType))
	return nil
}

func (r *MfaFactorRepository) DeleteAllForSub(_ context.Context, sub string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := sub + "|"
	for key := range r.factors {
		if strings.HasPrefix(key, prefix) {
			delete(r.factors, key)
		}
	}
	return nil
}

func mfaFactorKey(sub string, factorType spec.MfaFactorType) string {
	return sub + "|" + string(factorType)
}

func cloneMfaFactor(factor *spec.MfaFactor) *spec.MfaFactor {
	if factor == nil {
		return nil
	}
	out := *factor
	if factor.Secret != nil {
		secret := *factor.Secret
		out.Secret = &secret
	}
	if factor.Label != nil {
		label := *factor.Label
		out.Label = &label
	}
	if factor.LastUsedAt != nil {
		lastUsedAt := *factor.LastUsedAt
		out.LastUsedAt = &lastUsedAt
	}
	return &out
}

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

func defaultTenant(tenantID *string) {
	if *tenantID == "" {
		*tenantID = spec.DefaultTenantID
	}
}

func tenantKey(tenantID, id string) string {
	if tenantID == "" {
		tenantID = spec.DefaultTenantID
	}
	return tenantID + "|" + id
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
	defaultTenant(&sess.TenantID)
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

func (s *SessionStore) DeleteAllForSub(_ context.Context, sub string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.sessions {
		if sess.Sub == sub {
			delete(s.sessions, id)
		}
	}
	return nil
}
