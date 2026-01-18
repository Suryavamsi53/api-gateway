package repository

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreTokenBucket(t *testing.T) {
	mem := NewMemoryStore()
	ctx := context.Background()

	// Test: First request should succeed
	allowed, remaining, err := mem.TokenBucket(ctx, "user:1", 10, 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("first request should be allowed")
	}
	if remaining != 9 {
		t.Fatalf("expected remaining 9, got %d", remaining)
	}

	// Test: Rapid fire within capacity
	for i := 0; i < 9; i++ {
		ok, _, _ := mem.TokenBucket(ctx, "user:1", 10, 10, 1)
		if !ok {
			t.Fatalf("request %d should be allowed", i+2)
		}
	}

	// Test: Exceed capacity
	allowed, _, err = mem.TokenBucket(ctx, "user:1", 10, 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("11th request should be denied")
	}

	// Test: Refill after delay
	time.Sleep(100 * time.Millisecond)
	allowed, _, err = mem.TokenBucket(ctx, "user:1", 10, 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("should have refilled after delay")
	}
}

func TestMemoryStoreSlidingWindow(t *testing.T) {
	mem := NewMemoryStore()
	ctx := context.Background()

	// Test: First few events within window
	for i := 0; i < 5; i++ {
		count, err := mem.SlidingWindow(ctx, "endpoint:/api/users", 1000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != int64(i+1) {
			t.Fatalf("expected count %d, got %d", i+1, count)
		}
	}

	// Test: Events outside window are cleaned up
	time.Sleep(1100 * time.Millisecond)
	count, err := mem.SlidingWindow(ctx, "endpoint:/api/users", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1 after window expiry, got %d", count)
	}
}
