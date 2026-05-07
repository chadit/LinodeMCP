package linode

import (
	"sync"
	"time"
)

// circuitState is the breaker's lifecycle position. Closed is the steady
// state (requests pass), open is the tripped state (requests rejected),
// half-open lets exactly one probe through after the cooldown elapses.
type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// CircuitBreaker is a counting breaker: trips after consecutiveFailures
// reaches threshold, stays open for timeout, then admits one probe.
// A successful probe closes; a failing probe re-opens the timer.
//
// Threshold of 0 disables the breaker entirely (Allow always returns nil).
type CircuitBreaker struct {
	mu                  sync.Mutex
	state               circuitState
	consecutiveFailures int
	openedAt            time.Time
	threshold           int
	timeout             time.Duration
}

// NewCircuitBreaker constructs a breaker. A non-positive threshold yields a
// disabled breaker so callers don't have to special-case it.
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     circuitClosed,
		threshold: threshold,
		timeout:   timeout,
	}
}

// Allow reports whether a request may proceed. When the breaker is open and
// the cooldown has elapsed, Allow transitions to half-open and returns nil
// for that single probe; subsequent concurrent calls during the probe are
// rejected with ErrCircuitOpen.
func (cb *CircuitBreaker) Allow() error {
	if cb == nil || cb.threshold <= 0 {
		return nil
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return nil
	case circuitOpen:
		if time.Since(cb.openedAt) >= cb.timeout {
			cb.state = circuitHalfOpen

			return nil
		}

		return ErrCircuitOpen
	case circuitHalfOpen:
		// A probe is already in flight; reject anything else until it resolves.
		return ErrCircuitOpen
	}

	return nil
}

// RecordSuccess closes the breaker and resets the failure counter. Called
// after any successful upstream call (including the half-open probe).
func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil || cb.threshold <= 0 {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	cb.state = circuitClosed
}

// RecordFailure increments the failure counter and trips the breaker once
// the threshold is hit. Callers should only invoke this for upstream-health
// failures (5xx, network, 429). Auth errors and caller-driven cancellations
// are not the breaker's job.
func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil || cb.threshold <= 0 {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++
	if cb.consecutiveFailures >= cb.threshold {
		cb.state = circuitOpen
		cb.openedAt = time.Now()
	}
}
