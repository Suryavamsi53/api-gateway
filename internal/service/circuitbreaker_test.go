package service

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 1*time.Second)

	err := cb.Call(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error in closed state, got %v", err)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 1*time.Second)

	failErr := errors.New("service error")

	// Record 3 failures to trigger open
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error {
			return failErr
		})
	}

	if cb.GetState() != StateOpen {
		t.Errorf("expected state Open after 3 failures, got %s", cb.GetState())
	}

	// Try to call - should fail with circuit breaker error
	err := cb.Call(func() error {
		return nil
	})

	if err != ErrCircuitBreakerOpen {
		t.Errorf("expected ErrCircuitBreakerOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error {
			return errors.New("fail")
		})
	}

	if cb.GetState() != StateOpen {
		t.Errorf("expected state Open, got %s", cb.GetState())
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next call should transition to half-open
	_ = cb.Call(func() error {
		return nil
	})

	if cb.GetState() != StateHalfOpen {
		t.Errorf("expected state HalfOpen, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error {
			return errors.New("fail")
		})
	}

	// Wait for timeout to half-open
	time.Sleep(150 * time.Millisecond)

	// Fail in half-open state
	_ = cb.Call(func() error {
		return errors.New("fail")
	})

	if cb.GetState() != StateOpen {
		t.Errorf("expected state Open after failure in HalfOpen, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)

	// Open the circuit
	for i := 0; i < 3; i++ {
		_ = cb.Call(func() error {
			return errors.New("fail")
		})
	}

	// Wait for timeout to half-open
	time.Sleep(150 * time.Millisecond)

	// Succeed 2 times to close
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return nil
		})
	}

	if cb.GetState() != StateClosed {
		t.Errorf("expected state Closed after 2 successes in HalfOpen, got %s", cb.GetState())
	}
}

func TestCircuitBreakerPool(t *testing.T) {
	pool := NewCircuitBreakerPool(2, 1, 100*time.Millisecond)

	cb1 := pool.Get("service1")
	cb2 := pool.Get("service1")

	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance for same service")
	}

	cb3 := pool.Get("service2")
	if cb1 == cb3 {
		t.Error("expected different circuit breaker for different service")
	}
}

func TestCircuitBreakerPool_GetMetrics(t *testing.T) {
	pool := NewCircuitBreakerPool(2, 1, 100*time.Millisecond)

	cb := pool.Get("service1")
	_ = cb.Call(func() error {
		return errors.New("fail")
	})

	metrics := pool.GetMetrics()
	if metrics["service1"].FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", metrics["service1"].FailureCount)
	}
}

func TestCircuitBreakerPool_Reset(t *testing.T) {
	pool := NewCircuitBreakerPool(2, 1, 100*time.Millisecond)

	cb := pool.Get("service1")
	// Trigger open state
	for i := 0; i < 2; i++ {
		_ = cb.Call(func() error {
			return errors.New("fail")
		})
	}

	if cb.GetState() != StateOpen {
		t.Fatalf("expected Open state, got %s", cb.GetState())
	}

	pool.Reset("service1")

	if cb.GetState() != StateClosed {
		t.Errorf("expected Closed state after reset, got %s", cb.GetState())
	}
}

func TestCircuitBreaker_GetMetrics(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 1*time.Second)

	// Record a failure first
	_ = cb.Call(func() error { return errors.New("fail") })

	metrics := cb.GetMetrics()

	if metrics.State != StateClosed {
		t.Errorf("expected state Closed, got %s", metrics.State)
	}
	if metrics.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", metrics.FailureCount)
	}
	// Success count resets after failure, so it should be 0
	if metrics.SuccessCount != 0 {
		t.Errorf("expected 0 success after failure, got %d", metrics.SuccessCount)
	}
}

func TestCircuitBreaker_MaxConcurrentRequests(t *testing.T) {
	cb := NewCircuitBreaker(10, 5, 1*time.Second)
	cb.maxConcurrentRequests = 2

	// Open circuit to test half-open max concurrent
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.currentRequests = 2 // Set to max
	cb.mu.Unlock()

	// Third request should be rejected since we already have 2 concurrent
	err := cb.Call(func() error {
		return nil
	})

	if err != ErrCircuitBreakerOpen {
		t.Errorf("expected ErrCircuitBreakerOpen for concurrent limit, got %v", err)
	}
}
