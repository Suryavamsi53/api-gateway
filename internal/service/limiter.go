package service

import (
	"context"
	"fmt"

	"api-gateway/internal/repository"
)

// AlgorithmType enumerates supported algorithms.
type AlgorithmType string

const (
	TokenBucketAlg   AlgorithmType = "tokenbucket"
	SlidingWindowAlg AlgorithmType = "slidingwindow"
)

// Policy describes a rate limit policy.
type Policy struct {
	Algorithm AlgorithmType
	Capacity  int64
	Rate      float64 // tokens per second for token bucket
	WindowMs  int64   // window size for sliding window, milliseconds
	Limit     int64   // limit for sliding window
}

// Limiter provides rate-limiting evaluation.
type Limiter struct {
	store repository.Store
}

// NewLimiter constructs a Limiter.
func NewLimiter(s repository.Store) *Limiter {
	return &Limiter{store: s}
}

// Allow evaluates whether an event identified by key is allowed.
// It returns allowed and remaining quota (where applicable).
func (l *Limiter) Allow(ctx context.Context, key string, p Policy) (bool, int64, error) {
	switch p.Algorithm {
	case TokenBucketAlg:
		// tokens requested = 1
		allowed, remaining, err := l.store.TokenBucket(ctx, "tb:"+key, p.Capacity, p.Rate, 1)
		if err != nil {
			return false, 0, err
		}
		return allowed, remaining, nil
	case SlidingWindowAlg:
		count, err := l.store.SlidingWindow(ctx, "sw:"+key, p.WindowMs)
		if err != nil {
			return false, 0, err
		}
		allowed := count <= p.Limit
		remaining := p.Limit - count
		if remaining < 0 {
			remaining = 0
		}
		return allowed, remaining, nil
	default:
		return false, 0, fmt.Errorf("unknown algorithm %s", p.Algorithm)
	}
}
