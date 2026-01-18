package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// TestRedisStoreTokenBucket tests Redis-backed token bucket with miniredis.
func TestRedisStoreTokenBucket(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run failed: %v", err)
	}
	defer mr.Close()

	store, err := NewRedisStore(mr.Addr())
	if err != nil {
		t.Fatalf("failed to create redis store: %v", err)
	}

	ctx := context.Background()

	// First request succeeds
	allowed, _, err := store.TokenBucket(ctx, "user:1", 10, 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("first request should be allowed")
	}

	// Rapid fire within capacity
	for i := 0; i < 9; i++ {
		ok, _, _ := store.TokenBucket(ctx, "user:1", 10, 10, 1)
		if !ok {
			t.Fatalf("request %d should be allowed", i+2)
		}
	}

	// Exceed capacity
	allowed, _, err = store.TokenBucket(ctx, "user:1", 10, 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("11th request should be denied")
	}
}

// TestRedisStoreSlidingWindow tests Redis-backed sliding window with miniredis.
func TestRedisStoreSlidingWindow(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run failed: %v", err)
	}
	defer mr.Close()

	store, err := NewRedisStore(mr.Addr())
	if err != nil {
		t.Fatalf("failed to create redis store: %v", err)
	}

	ctx := context.Background()

	// Add events within window
	for i := 0; i < 5; i++ {
		count, err := store.SlidingWindow(ctx, "endpoint:/api/users", 1000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != int64(i+1) {
			t.Fatalf("expected count %d, got %d", i+1, count)
		}
	}

	// Verify count is 5
	count, _ := store.SlidingWindow(ctx, "endpoint:/api/users", 1000)
	if count != 5 {
		t.Fatalf("expected count 5, got %d", count)
	}
}

// BenchmarkRedisTokenBucket benchmarks Redis token bucket performance.
func BenchmarkRedisTokenBucket(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis run failed: %v", err)
	}
	defer mr.Close()

	store, _ := NewRedisStore(mr.Addr())
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.TokenBucket(ctx, "bench:key", 100, 100, 1)
	}
}

// BenchmarkRedisStreamConcurrent benchmarks concurrent Redis operations.
func BenchmarkRedisStreamConcurrent(b *testing.B) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatalf("miniredis run failed: %v", err)
	}
	defer mr.Close()

	store, _ := NewRedisStore(mr.Addr())
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			store.TokenBucket(ctx, "key:"+string(rune(i%10)), 100, 100, 1)
			i++
		}
	})
}
