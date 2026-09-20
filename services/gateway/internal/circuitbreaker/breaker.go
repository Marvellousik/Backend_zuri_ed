// Package circuitbreaker provides circuit breaker pattern implementation for resilient HTTP calls.
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed allows requests to pass through
	StateClosed State = iota
	// StateOpen rejects all requests immediately
	StateOpen
	// StateHalfOpen allows one test request to pass through
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	// MaxFailures before opening the circuit
	MaxFailures uint32
	// Timeout duration to wait before transitioning from OPEN to HALF_OPEN
	Timeout time.Duration
	// OnStateChange is called when the circuit breaker state changes
	OnStateChange func(from, to State)
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxFailures: 3,
		Timeout:     60 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config        Config
	state         State
	failures      uint32
	lastFailureAt time.Time
	mutex         sync.RWMutex
}

// ErrCircuitBreakerOpen is returned when the circuit is open
var ErrCircuitBreakerOpen = errors.New("circuit breaker is open")

// New creates a new circuit breaker
func New(config Config) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// Execute runs the given function if the circuit allows it
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	state := cb.currentState()

	if state == StateOpen {
		return ErrCircuitBreakerOpen
	}

	err := fn()
	cb.recordResult(state, err)
	return err
}

// currentState returns the current state, handling transitions
func (cb *CircuitBreaker) currentState() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()

	// Check if we should transition from OPEN to HALF_OPEN
	if cb.state == StateOpen && now.Sub(cb.lastFailureAt) > cb.config.Timeout {
		cb.transitionTo(StateHalfOpen)
	}

	return cb.state
}

// recordResult records the result of a function execution
func (cb *CircuitBreaker) recordResult(beforeState State, err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err == nil {
		// Success
		if beforeState == StateHalfOpen {
			// Transition from HALF_OPEN to CLOSED on success
			cb.transitionTo(StateClosed)
		}
		cb.failures = 0
		return
	}

	// Failure
	cb.failures++
	cb.lastFailureAt = time.Now()

	if beforeState == StateHalfOpen {
		// Transition from HALF_OPEN to OPEN on failure
		cb.transitionTo(StateOpen)
	} else if cb.failures >= cb.config.MaxFailures {
		// Transition from CLOSED to OPEN after max failures
		cb.transitionTo(StateOpen)
	}
}

// transitionTo changes the state and calls the callback if configured
func (cb *CircuitBreaker) transitionTo(newState State) {
	oldState := cb.state
	if oldState != newState {
		cb.state = newState
		if cb.config.OnStateChange != nil {
			cb.config.OnStateChange(oldState, newState)
		}
	}
}

// Metrics returns current metrics for monitoring
func (cb *CircuitBreaker) Metrics() Metrics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return Metrics{
		State:         cb.state.String(),
		Failures:      cb.failures,
		LastFailureAt: cb.lastFailureAt,
	}
}

// Metrics holds circuit breaker metrics
type Metrics struct {
	State         string    `json:"state"`
	Failures      uint32    `json:"failures"`
	LastFailureAt time.Time `json:"last_failure_at,omitempty"`
}

// String returns a string representation of metrics
func (m Metrics) String() string {
	return fmt.Sprintf("State: %s, Failures: %d", m.State, m.Failures)
}
