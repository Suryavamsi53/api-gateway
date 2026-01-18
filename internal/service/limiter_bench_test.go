package service

import (
	"context"
	"testing"

	"api-gateway/internal/repository"
)

// BenchmarkTokenBucketMemory benchmarks token bucket on memory store.
func BenchmarkTokenBucketMemory(b *testing.B) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: TokenBucketAlg, Capacity: 100, Rate: 100}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lim.Allow(ctx, "bench:key", policy)
	}
}

// BenchmarkSlidingWindowMemory benchmarks sliding window on memory store.
func BenchmarkSlidingWindowMemory(b *testing.B) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: SlidingWindowAlg, WindowMs: 1000, Limit: 100}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lim.Allow(ctx, "bench:key", policy)
	}
}

// BenchmarkConcurrentTokenBucket benchmarks concurrent token bucket access.
func BenchmarkConcurrentTokenBucket(b *testing.B) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: TokenBucketAlg, Capacity: 1000, Rate: 1000}
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lim.Allow(ctx, "bench:key:"+string(rune(i%100)), policy)
			i++
		}
	})
}

// BenchmarkConcurrentSlidingWindow benchmarks concurrent sliding window access.
func BenchmarkConcurrentSlidingWindow(b *testing.B) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: SlidingWindowAlg, WindowMs: 1000, Limit: 1000}
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lim.Allow(ctx, "bench:key:"+string(rune(i%100)), policy)
			i++
		}
	})
}
