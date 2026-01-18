package service

import (
	"context"
	"testing"

	"api-gateway/internal/repository"
)

func TestTokenBucketAlgorithm(t *testing.T) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: TokenBucketAlg, Capacity: 5, Rate: 5}

	tests := []struct {
		name    string
		allowed bool
	}{
		{"1st", true},
		{"2nd", true},
		{"3rd", true},
		{"4th", true},
		{"5th", true},
		{"6th", false},
	}

	for i, tt := range tests {
		ok, _, err := lim.Allow(context.Background(), "key1", policy)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if ok != tt.allowed {
			t.Fatalf("test %d (%s): expected allowed=%v, got %v", i, tt.name, tt.allowed, ok)
		}
	}
}

func TestSlidingWindowAlgorithm(t *testing.T) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: SlidingWindowAlg, WindowMs: 1000, Limit: 3}

	tests := []struct {
		name    string
		allowed bool
	}{
		{"1st", true},
		{"2nd", true},
		{"3rd", true},
		{"4th", false},
	}

	for i, tt := range tests {
		ok, _, err := lim.Allow(context.Background(), "key2", policy)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if ok != tt.allowed {
			t.Fatalf("test %d (%s): expected allowed=%v, got %v", i, tt.name, tt.allowed, ok)
		}
	}
}

func TestMultipleKeys(t *testing.T) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: TokenBucketAlg, Capacity: 2, Rate: 2}

	// User 1: consume 2 tokens
	ok1, _, _ := lim.Allow(context.Background(), "user:1", policy)
	ok2, _, _ := lim.Allow(context.Background(), "user:1", policy)
	if !ok1 || !ok2 {
		t.Fatal("user 1 first 2 requests should succeed")
	}

	// User 1: third request should fail
	ok3, _, _ := lim.Allow(context.Background(), "user:1", policy)
	if ok3 {
		t.Fatal("user 1 third request should fail")
	}

	// User 2: should have independent quota
	ok4, _, _ := lim.Allow(context.Background(), "user:2", policy)
	ok5, _, _ := lim.Allow(context.Background(), "user:2", policy)
	if !ok4 || !ok5 {
		t.Fatal("user 2 should have independent quota")
	}
}
