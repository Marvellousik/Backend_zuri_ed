// Package proxy provides reverse proxy and circuit breaker functionality.
package proxy

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed State = iota    // Normal operation
	StateOpen                   // Failing, rejecting requests
	StateHalfOpen               // Testing if service recovered
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	name          string
	threshold     int           // Number of failures before opening
	timeout       time.Duration // How long to stay open
	
	state         State
	failures      int
	lastFailure   time.Time
	halfOpenCalls int
	
	mu sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:      name,
		threshold: threshold,
		timeout:   timeout,
		state:     StateClosed,
	}
}

// Execute runs the given function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	
	// Check if we should transition from Open to HalfOpen
	if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.timeout {
		cb.state = StateHalfOpen
		cb.halfOpenCalls = 0
	}
	
	// If circuit is open, reject immediately
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	
	// Track half-open calls
	if cb.state == StateHalfOpen {
		cb.halfOpenCalls++
	}
	
	cb.mu.Unlock()
	
	// Execute the function
	err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.recordFailure()
		return err
	}
	
	cb.recordSuccess()
	return nil
}

// recordFailure records a failure and potentially opens the circuit.
func (cb *CircuitBreaker) recordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()
	
	if cb.state == StateHalfOpen {
		// Failed in half-open state, go back to open
		cb.state = StateOpen
		return
	}
	
	if cb.failures >= cb.threshold {
		cb.state = StateOpen
	}
}

// recordSuccess records a success and potentially closes the circuit.
func (cb *CircuitBreaker) recordSuccess() {
	if cb.state == StateHalfOpen {
		// Success in half-open, close the circuit
		cb.state = StateClosed
		cb.failures = 0
		cb.halfOpenCalls = 0
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Metrics returns current metrics for monitoring.
func (cb *CircuitBreaker) Metrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	return map[string]interface{}{
		"name":           cb.name,
		"state":          cb.state.String(),
		"failures":       cb.failures,
		"last_failure":   cb.lastFailure,
		"half_open_calls": cb.halfOpenCalls,
	}
}

// String returns string representation of state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
