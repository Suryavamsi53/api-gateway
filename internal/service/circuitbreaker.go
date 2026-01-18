package service

import (
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	StateClosed   CircuitState = "closed"
	StateOpen     CircuitState = "open"
	StateHalfOpen CircuitState = "half-open"
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu                    sync.RWMutex
	state                 CircuitState
	failureCount          int
	successCount          int
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	lastFailureTime       time.Time
	maxConcurrentRequests int
	currentRequests       int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:                 StateClosed,
		failureThreshold:      failureThreshold,
		successThreshold:      successThreshold,
		timeout:               timeout,
		maxConcurrentRequests: 100,
	}
}

// Call executes a function if the circuit allows it
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	// Check state
	if cb.state == StateOpen {
		// Check if timeout has passed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
		} else {
			cb.mu.Unlock()
			return ErrCircuitBreakerOpen
		}
	}

	// Check max concurrent requests (for half-open state)
	if cb.state == StateHalfOpen && cb.currentRequests >= cb.maxConcurrentRequests {
		cb.mu.Unlock()
		return ErrCircuitBreakerOpen
	}

	cb.currentRequests++
	cb.mu.Unlock()

	// Execute function
	defer func() {
		cb.mu.Lock()
		cb.currentRequests--
		cb.mu.Unlock()
	}()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return err
}

// recordFailure records a failure
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	cb.successCount = 0

	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
	}
}

// recordSuccess records a success
func (cb *CircuitBreaker) recordSuccess() {
	cb.failureCount = 0
	cb.successCount++

	if cb.state == StateHalfOpen && cb.successCount >= cb.successThreshold {
		cb.state = StateClosed
		cb.successCount = 0
	}
}

// GetState returns the current state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() CircuitMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitMetrics{
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		CurrentRequests: cb.currentRequests,
	}
}

// CircuitMetrics contains circuit breaker metrics
type CircuitMetrics struct {
	State           CircuitState
	FailureCount    int
	SuccessCount    int
	CurrentRequests int
}

// CircuitBreakerPool manages multiple circuit breakers
type CircuitBreakerPool struct {
	mu        sync.RWMutex
	breakers  map[string]*CircuitBreaker
	failureTh int
	successTh int
	timeout   time.Duration
}

// NewCircuitBreakerPool creates a new circuit breaker pool
func NewCircuitBreakerPool(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreakerPool {
	return &CircuitBreakerPool{
		breakers:  make(map[string]*CircuitBreaker),
		failureTh: failureThreshold,
		successTh: successThreshold,
		timeout:   timeout,
	}
}

// Get returns or creates a circuit breaker for a service
func (cbp *CircuitBreakerPool) Get(service string) *CircuitBreaker {
	cbp.mu.Lock()
	defer cbp.mu.Unlock()

	if cb, exists := cbp.breakers[service]; exists {
		return cb
	}

	cb := NewCircuitBreaker(cbp.failureTh, cbp.successTh, cbp.timeout)
	cbp.breakers[service] = cb
	return cb
}

// GetAll returns all circuit breakers
func (cbp *CircuitBreakerPool) GetAll() map[string]*CircuitBreaker {
	cbp.mu.RLock()
	defer cbp.mu.RUnlock()
	return cbp.breakers
}

// GetMetrics returns metrics for all circuit breakers
func (cbp *CircuitBreakerPool) GetMetrics() map[string]CircuitMetrics {
	cbp.mu.RLock()
	breakers := make(map[string]*CircuitBreaker)
	for k, v := range cbp.breakers {
		breakers[k] = v
	}
	cbp.mu.RUnlock()

	metrics := make(map[string]CircuitMetrics)
	for service, cb := range breakers {
		metrics[service] = cb.GetMetrics()
	}
	return metrics
}

// Reset resets a circuit breaker
func (cbp *CircuitBreakerPool) Reset(service string) {
	cbp.mu.Lock()
	defer cbp.mu.Unlock()

	if cb, exists := cbp.breakers[service]; exists {
		cb.mu.Lock()
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
		cb.mu.Unlock()
	}
}

// ResetAll resets all circuit breakers
func (cbp *CircuitBreakerPool) ResetAll() {
	cbp.mu.RLock()
	breakers := make(map[string]*CircuitBreaker)
	for k, v := range cbp.breakers {
		breakers[k] = v
	}
	cbp.mu.RUnlock()

	for _, cb := range breakers {
		cb.mu.Lock()
		cb.state = StateClosed
		cb.failureCount = 0
		cb.successCount = 0
		cb.mu.Unlock()
	}
}

// Custom errors
var (
	ErrCircuitBreakerOpen = NewError("circuit_breaker_open", "circuit breaker is open")
)
