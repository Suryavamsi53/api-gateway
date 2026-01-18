package middleware

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

const (
	// MaxRequestSize limits request body size to 10MB
	MaxRequestSize = 10 * 1024 * 1024
)

// RequestSizeLimit enforces maximum request body size.
func RequestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				log.Warn().
					Int64("content_length", r.ContentLength).
					Int64("max_size", maxBytes).
					Msg("request body too large")
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// CircuitBreakerState tracks circuit breaker state.
type CircuitBreakerState int

const (
	Closed CircuitBreakerState = iota
	Open
	HalfOpen
)

// CircuitBreaker implements a simple circuit breaker for downstream errors.
type CircuitBreaker struct {
	state           CircuitBreakerState
	failureCount    int
	failureThreshold int
	resetTimeout    int // seconds
	lastFailureTime  int64
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int, resetTimeout int) *CircuitBreaker {
	return &CircuitBreaker{
		state:            Closed,
		failureThreshold: threshold,
		resetTimeout:     resetTimeout,
	}
}

// RecordSuccess resets the circuit breaker.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.state = Closed
	cb.failureCount = 0
}

// RecordFailure increments failure count and trips the circuit if threshold exceeded.
func (cb *CircuitBreaker) RecordFailure() bool {
	cb.failureCount++
	if cb.failureCount >= cb.failureThreshold {
		cb.state = Open
		return true
	}
	return false
}

// IsOpen returns whether the circuit is open.
func (cb *CircuitBreaker) IsOpen() bool {
	return cb.state == Open
}

// Middleware wraps a handler with circuit breaker protection.
func (cb *CircuitBreaker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cb.IsOpen() {
			log.Warn().Msg("circuit breaker open")
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}
