package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ra-idp-go/internal/spec"
)

func TestAuthorizationCodeRedeemIsAtomic(t *testing.T) {
	store := NewAuthorizationCodeStore()
	code := &spec.AuthorizationCodeRecord{Code: "code", State: spec.AuthCodeRecordIssued}
	if err := store.Save(context.Background(), code); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec, err := store.Redeem(context.Background(), "code", time.Now())
			if err != nil {
				t.Errorf("redeem: %v", err)
			}
			if rec != nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful redeems=%d, want 1", successes.Load())
	}
}

func TestDeviceCodeExchangeIsAtomic(t *testing.T) {
	store := NewDeviceCodeStore()
	rec := &spec.DeviceAuthorization{
		DeviceCodeHash: "hash", UserCode: "CODE", State: spec.DeviceFlowApproved,
	}
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exchanged, err := store.Exchange(context.Background(), "hash")
			if err != nil {
				t.Errorf("exchange: %v", err)
			}
			if exchanged != nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful exchanges=%d, want 1", successes.Load())
	}
}

func TestReplayStoreAcceptsJTIOnce(t *testing.T) {
	store := NewDpopReplayStore()
	now := time.Now()
	first, err := store.RecordIfNew(context.Background(), "jti", 60, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.RecordIfNew(context.Background(), "jti", 60, now)
	if err != nil {
		t.Fatal(err)
	}
	if !first || second {
		t.Fatalf("first=%v second=%v", first, second)
	}
}
