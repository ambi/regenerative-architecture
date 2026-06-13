package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	authports "ra-idp-go/internal/authentication/ports"
)

func TestPasswordResetTokenStoreInvalidatesPreviousTokenForSubject(t *testing.T) {
	store := NewPasswordResetTokenStore()
	now := time.Now().UTC()
	for _, hash := range []string{"old", "new"} {
		if err := store.Save(context.Background(), authports.PasswordResetTokenRecord{
			Sub: "user", TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	old, _ := store.Consume(context.Background(), "old", now)
	next, _ := store.Consume(context.Background(), "new", now)
	if old != nil || next == nil {
		t.Fatalf("old=%#v new=%#v", old, next)
	}
}

func TestPasswordResetTokenStoreConsumeSucceedsOnceConcurrently(t *testing.T) {
	store := NewPasswordResetTokenStore()
	now := time.Now().UTC()
	if err := store.Save(context.Background(), authports.PasswordResetTokenRecord{
		Sub: "user", TokenHash: "token", CreatedAt: now, ExpiresAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan *authports.PasswordResetTokenRecord, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, _ := store.Consume(context.Background(), "token", now)
			results <- record
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for record := range results {
		if record != nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumes=%d, want 1", successes)
	}
}
