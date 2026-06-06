package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ra-idp-go/internal/spec"

	goredis "github.com/redis/go-redis/v9"
)

func Open(ctx context.Context, rawURL string) (*goredis.Client, error) {
	options, err := goredis.ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func setJSON(ctx context.Context, client goredis.Cmdable, key string, value any, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return client.Set(ctx, key, payload, ttl).Err()
}

func getJSON(ctx context.Context, client goredis.Cmdable, key string, out any) error {
	payload, err := client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, out)
}

func ttlUntil(expiresAt time.Time) time.Duration {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return time.Millisecond
	}
	return ttl
}

type AuthorizationRequestStore struct{ Client *goredis.Client }

func (s *AuthorizationRequestStore) Save(ctx context.Context, req *spec.AuthorizationRequest) error {
	return setJSON(ctx, s.Client, "idp:authreq:"+req.ID, req, ttlUntil(req.ExpiresAt))
}

func (s *AuthorizationRequestStore) Find(ctx context.Context, id string) (*spec.AuthorizationRequest, error) {
	var req spec.AuthorizationRequest
	if err := getJSON(ctx, s.Client, "idp:authreq:"+id, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, nil
	}
	return &req, nil
}

func (s *AuthorizationRequestStore) UpdateState(ctx context.Context, id string, state spec.AuthorizationCodeFlowState) error {
	return s.update(ctx, id, func(req *spec.AuthorizationRequest) error {
		next, err := spec.TransitionAuthorizationCodeFlow(req.State, eventForTargetState(state))
		if err != nil {
			return err
		}
		req.State = next
		return nil
	})
}

func (s *AuthorizationRequestStore) AttachSubject(ctx context.Context, id, sub string, authTime int64) error {
	return s.update(ctx, id, func(req *spec.AuthorizationRequest) error {
		req.Sub, req.AuthTime = &sub, &authTime
		return nil
	})
}

func (s *AuthorizationRequestStore) update(
	ctx context.Context,
	id string,
	change func(*spec.AuthorizationRequest) error,
) error {
	key := "idp:authreq:" + id
	return s.Client.Watch(ctx, func(tx *goredis.Tx) error {
		var req spec.AuthorizationRequest
		if err := getJSON(ctx, tx, key, &req); err != nil {
			return err
		}
		if req.ID == "" {
			return fmt.Errorf("authorization request %q not found", id)
		}
		if err := change(&req); err != nil {
			return err
		}
		payload, err := json.Marshal(&req)
		if err != nil {
			return err
		}
		ttl, err := tx.TTL(ctx, key).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, key, payload, ttl)
			return nil
		})
		return err
	}, key)
}

func eventForTargetState(to spec.AuthorizationCodeFlowState) spec.AuthorizationCodeFlowEvent {
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
	default:
		return "unknown"
	}
}

type AuthorizationCodeStore struct{ Client *goredis.Client }

func (s *AuthorizationCodeStore) Save(ctx context.Context, rec *spec.AuthorizationCodeRecord) error {
	return setJSON(ctx, s.Client, "idp:code:"+rec.Code, rec, ttlUntil(rec.ExpiresAt))
}

func (s *AuthorizationCodeStore) Find(ctx context.Context, code string) (*spec.AuthorizationCodeRecord, error) {
	var rec spec.AuthorizationCodeRecord
	if err := getJSON(ctx, s.Client, "idp:code:"+code, &rec); err != nil {
		return nil, err
	}
	if rec.Code == "" {
		return nil, nil
	}
	return &rec, nil
}

var redeemCode = goredis.NewScript(`
local payload = redis.call('GET', KEYS[1])
if not payload then return false end
local rec = cjson.decode(payload)
if rec.state ~= 'issued' then return false end
rec.state = 'redeemed'
rec.redeemed_at = ARGV[1]
redis.call('SET', KEYS[1], cjson.encode(rec), 'KEEPTTL')
return cjson.encode(rec)
`)

func (s *AuthorizationCodeStore) Redeem(ctx context.Context, code string, now time.Time) (*spec.AuthorizationCodeRecord, error) {
	result, err := redeemCode.Run(ctx, s.Client, []string{"idp:code:" + code}, now.UTC().Format(time.RFC3339Nano)).Text()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec spec.AuthorizationCodeRecord
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *AuthorizationCodeStore) LinkFamily(ctx context.Context, code, familyID string) error {
	key := "idp:code:" + code
	return s.Client.Watch(ctx, func(tx *goredis.Tx) error {
		var rec spec.AuthorizationCodeRecord
		if err := getJSON(ctx, tx, key, &rec); err != nil {
			return err
		}
		if rec.Code == "" {
			return errors.New("authorization code not found")
		}
		rec.IssuedFamilyID = &familyID
		payload, _ := json.Marshal(&rec)
		ttl, err := tx.TTL(ctx, key).Result()
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, key, payload, ttl)
			return nil
		})
		return err
	}, key)
}

type PARStore struct{ Client *goredis.Client }

func (s *PARStore) Save(ctx context.Context, rec *spec.PARRecord) error {
	return setJSON(ctx, s.Client, "idp:par:"+rec.RequestURI, rec, ttlUntil(rec.ExpiresAt))
}

func (s *PARStore) Find(ctx context.Context, uri string) (*spec.PARRecord, error) {
	var rec spec.PARRecord
	if err := getJSON(ctx, s.Client, "idp:par:"+uri, &rec); err != nil {
		return nil, err
	}
	if rec.RequestURI == "" {
		return nil, nil
	}
	return &rec, nil
}

var consumePAR = goredis.NewScript(`
local payload = redis.call('GET', KEYS[1])
if not payload then return false end
local rec = cjson.decode(payload)
if rec.used then return false end
rec.used = true
redis.call('SET', KEYS[1], cjson.encode(rec), 'KEEPTTL')
return cjson.encode(rec)
`)

func (s *PARStore) Consume(ctx context.Context, uri string) (*spec.PARRecord, error) {
	result, err := consumePAR.Run(ctx, s.Client, []string{"idp:par:" + uri}).Text()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec spec.PARRecord
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

type DeviceCodeStore struct{ Client *goredis.Client }

func (s *DeviceCodeStore) Save(ctx context.Context, rec *spec.DeviceAuthorization) error {
	ttl := ttlUntil(rec.ExpiresAt)
	pipe := s.Client.TxPipeline()
	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	pipe.Set(ctx, "idp:device:"+rec.DeviceCodeHash, payload, ttl)
	pipe.Set(ctx, "idp:device:uc:"+rec.UserCode, rec.DeviceCodeHash, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *DeviceCodeStore) FindByDeviceCodeHash(ctx context.Context, hash string) (*spec.DeviceAuthorization, error) {
	var rec spec.DeviceAuthorization
	if err := getJSON(ctx, s.Client, "idp:device:"+hash, &rec); err != nil {
		return nil, err
	}
	if rec.DeviceCodeHash == "" {
		return nil, nil
	}
	return &rec, nil
}

func (s *DeviceCodeStore) FindByUserCode(ctx context.Context, code string) (*spec.DeviceAuthorization, error) {
	hash, err := s.Client.Get(ctx, "idp:device:uc:"+code).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.FindByDeviceCodeHash(ctx, hash)
}

func (s *DeviceCodeStore) Update(ctx context.Context, rec *spec.DeviceAuthorization) error {
	key := "idp:device:" + rec.DeviceCodeHash
	ttl, err := s.Client.TTL(ctx, key).Result()
	if err != nil {
		return err
	}
	return setJSON(ctx, s.Client, key, rec, ttl)
}

var exchangeDevice = goredis.NewScript(`
local payload = redis.call('GET', KEYS[1])
if not payload then return false end
local rec = cjson.decode(payload)
if rec.state ~= 'approved' then return false end
rec.state = 'exchanged'
redis.call('SET', KEYS[1], cjson.encode(rec), 'KEEPTTL')
return cjson.encode(rec)
`)

func (s *DeviceCodeStore) Exchange(ctx context.Context, hash string) (*spec.DeviceAuthorization, error) {
	result, err := exchangeDevice.Run(ctx, s.Client, []string{"idp:device:" + hash}).Text()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec spec.DeviceAuthorization
	if err := json.Unmarshal([]byte(result), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

type ReplayStore struct {
	Client *goredis.Client
	Prefix string
}

func (s *ReplayStore) RecordIfNew(ctx context.Context, jti string, seconds int, _ time.Time) (bool, error) {
	return s.Client.SetNX(ctx, s.Prefix+jti, "1", time.Duration(seconds)*time.Second).Result()
}

type SessionStore struct{ Client *goredis.Client }

func (s *SessionStore) Save(ctx context.Context, session *spec.LoginSession) error {
	return setJSON(ctx, s.Client, "idp:session:"+session.ID, session, ttlUntil(session.ExpiresAt))
}

func (s *SessionStore) Find(ctx context.Context, id string) (*spec.LoginSession, error) {
	var session spec.LoginSession
	if err := getJSON(ctx, s.Client, "idp:session:"+id, &session); err != nil {
		return nil, err
	}
	if session.ID == "" {
		return nil, nil
	}
	return &session, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	return s.Client.Del(ctx, "idp:session:"+id).Err()
}
