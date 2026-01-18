package repository

import (
	"context"
	"sync"
	"time"
)

type memBucket struct {
	tokens int64
	last   int64
}

type memoryStore struct {
	mu      sync.Mutex
	buckets map[string]*memBucket
	sw      map[string][]int64
}

// NewMemoryStore returns an in-memory Store for local development/testing.
func NewMemoryStore() Store {
	return &memoryStore{
		buckets: make(map[string]*memBucket),
		sw:      make(map[string][]int64),
	}
}

func (m *memoryStore) TokenBucket(ctx context.Context, key string, capacity int64, refillRate float64, tokens int64) (bool, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UnixNano() / int64(time.Millisecond)
	b, ok := m.buckets[key]
	if !ok {
		b = &memBucket{tokens: capacity, last: now}
		m.buckets[key] = b
	}
	delta := now - b.last
	refill := int64(float64(delta) * (refillRate / 1000.0))
	if refill > 0 {
		b.tokens += refill
		if b.tokens > capacity {
			b.tokens = capacity
		}
		b.last = now
	}
	if b.tokens >= tokens {
		b.tokens -= tokens
		return true, b.tokens, nil
	}
	return false, b.tokens, nil
}

func (m *memoryStore) SlidingWindow(ctx context.Context, key string, windowMillis int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UnixNano() / int64(time.Millisecond)
	arr := m.sw[key]
	cutoff := now - windowMillis
	// remove old
	i := 0
	for ; i < len(arr); i++ {
		if arr[i] >= cutoff {
			break
		}
	}
	arr = arr[i:]
	arr = append(arr, now)
	m.sw[key] = arr
	return int64(len(arr)), nil
}
