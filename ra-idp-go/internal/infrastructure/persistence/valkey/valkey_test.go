package valkey

import (
	"context"
	"testing"
	"time"

	"ra-idp-go/internal/spec"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testClient(t *testing.T) *goredis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: server.Addr()})
}

func TestAuthorizationCodeRedeemOnce(t *testing.T) {
	client := testClient(t)
	store := &AuthorizationCodeStore{Client: client}
	rec := &spec.AuthorizationCodeRecord{
		Code: "code", State: spec.AuthCodeRecordIssued,
		ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := store.Save(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	first, err := store.Redeem(t.Context(), "code", time.Now())
	if err != nil || first == nil {
		t.Fatalf("first redeem: record=%v err=%v", first, err)
	}
	second, err := store.Redeem(t.Context(), "code", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal("authorization code redeemed twice")
	}
}

func TestPARAndReplayStoresAreSingleUse(t *testing.T) {
	client := testClient(t)
	parStore := &PARStore{Client: client}
	par := &spec.PARRecord{
		RequestURI: "urn:test", ClientID: "client",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := parStore.Save(context.Background(), par); err != nil {
		t.Fatal(err)
	}
	if first, _ := parStore.Consume(t.Context(), par.RequestURI); first == nil {
		t.Fatal("first PAR consume failed")
	}
	if second, _ := parStore.Consume(t.Context(), par.RequestURI); second != nil {
		t.Fatal("PAR consumed twice")
	}
	replay := &ReplayStore{Client: client, Prefix: "test:"}
	first, _ := replay.RecordIfNew(t.Context(), "jti", 60, time.Now())
	second, _ := replay.RecordIfNew(t.Context(), "jti", 60, time.Now())
	if !first || second {
		t.Fatalf("replay results first=%v second=%v", first, second)
	}
}

func TestDeviceExchangeOnce(t *testing.T) {
	client := testClient(t)
	store := &DeviceCodeStore{Client: client}
	rec := &spec.DeviceAuthorization{
		DeviceCodeHash: "hash", UserCode: "CODE", ClientID: "client",
		State: spec.DeviceFlowApproved, IntervalSeconds: 5,
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute),
	}
	if err := store.Save(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	if first, _ := store.Exchange(t.Context(), "hash"); first == nil {
		t.Fatal("first exchange failed")
	}
	if second, _ := store.Exchange(t.Context(), "hash"); second != nil {
		t.Fatal("device code exchanged twice")
	}
}
