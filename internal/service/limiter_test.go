package service

import (
	"context"
	"sync"
	"testing"

	"api-gateway/internal/repository"
)

func TestTokenBucketConcurrency(t *testing.T) {
	mem := repository.NewMemoryStore()
	lim := NewLimiter(mem)
	policy := Policy{Algorithm: TokenBucketAlg, Capacity: 10, Rate: 10}
	key := "testkey"

	var wg sync.WaitGroup
	allowedCount := 0
	mu := sync.Mutex{}
	N := 20
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ok, _, err := lim.Allow(context.Background(), key, policy)
			if err != nil {
				t.Error(err)
			}
			if ok {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowedCount > 10 {
		t.Fatalf("allowed more than capacity: %d", allowedCount)
	}
}
