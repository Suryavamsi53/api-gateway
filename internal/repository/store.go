package repository

import "context"

// Store defines methods used by rate-limit algorithms. Implementations must be concurrency-safe
// and support distributed atomic operations when backed by Redis.
type Store interface {
	// TokenBucket attempts to take `tokens` from the bucket identified by key.
	// Returns allowed, remaining tokens, error.
	TokenBucket(ctx context.Context, key string, capacity int64, refillRate float64, tokens int64) (bool, int64, error)

	// SlidingWindow increments event at current timestamp and returns count within window.
	SlidingWindow(ctx context.Context, key string, windowMillis int64) (int64, error)
}
