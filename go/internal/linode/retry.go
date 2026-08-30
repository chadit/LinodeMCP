package linode

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

type retryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	JitterEnabled bool
}

const (
	defaultMaxRetries    = 3
	defaultMaxDelay      = 30 * time.Second
	defaultBackoffFactor = 2.0
	jitterPercent        = 0.1
)

func defaultRetryConfig() retryConfig {
	return retryConfig{
		MaxRetries:    defaultMaxRetries,
		BaseDelay:     time.Second,
		MaxDelay:      defaultMaxDelay,
		BackoffFactor: defaultBackoffFactor,
		JitterEnabled: true,
	}
}

func (c *Client) executeWithoutRetry(ctx context.Context, operation string, run func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context canceled: %w", err)
	}

	err := run()
	if err == nil {
		c.circuit.RecordSuccess()

		return nil
	}

	if c.shouldRecordCircuitFailure(err) {
		c.circuit.RecordFailure()
	}

	return err
}

func (*Client) shouldRecordCircuitFailure(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.IsRateLimitError() || apiErr.IsServerError()
	}

	return isNetworkError(err) || isTimeoutError(err)
}

// executeWithRetry backs off and retries transient failures (network, timeout,
// 429, 5xx) behind the circuit breaker. The route primitives in route_call.go
// run under it, and pick executeWithoutRetry when a tool's contract declares
// that a replay would cost more than a duplicate request.
func (c *Client) executeWithRetry(ctx context.Context, operation string, retryFunc func() error) error {
	if err := c.circuit.Allow(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	var lastErr error

	var attempt int

	for attempt <= c.retryCfg.MaxRetries {
		if attempt > 0 {
			delay := c.delayForAttempt(attempt, lastErr)
			select {
			case <-ctx.Done():
				// Caller canceled; not an upstream-health signal.
				return fmt.Errorf("context canceled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		err := retryFunc()
		if err == nil {
			c.circuit.RecordSuccess()

			return nil
		}

		lastErr = err
		attempt++

		if !c.shouldRetry(err) {
			// Non-retryable (e.g. auth), not the breaker's concern.
			return err
		}
	}

	// Retries exhausted on a retryable failure: the signal the breaker tracks.
	c.circuit.RecordFailure()

	return fmt.Errorf("%s: %w", operation, lastErr)
}

// delayForAttempt waits out the upstream's Retry-After hint (typically on 429)
// when there is one, otherwise falls back to exponential backoff with jitter.
// The hint is clamped to MaxDelay so a hostile or buggy server can't ask us to
// wait an hour.
func (c *Client) delayForAttempt(attempt int, lastErr error) time.Duration {
	if apiErr, ok := errors.AsType[*APIError](lastErr); ok && apiErr.RetryAfter > 0 {
		hint := apiErr.RetryAfter
		if hint > c.retryCfg.MaxDelay {
			return c.retryCfg.MaxDelay
		}

		return hint
	}

	return c.calculateDelay(attempt)
}

func (c *Client) calculateDelay(attempt int) time.Duration {
	delay := float64(c.retryCfg.BaseDelay) * math.Pow(c.retryCfg.BackoffFactor, float64(attempt-1))

	if c.retryCfg.JitterEnabled {
		jitterMax := big.NewInt(int64(delay * jitterPercent))
		if jitterMax.Int64() > 0 {
			jitterBig, err := rand.Int(rand.Reader, jitterMax)
			if err != nil {
				return c.retryCfg.BaseDelay
			}

			jitter := float64(jitterBig.Int64())
			delay += jitter
		}
	}

	maxDelay := float64(c.retryCfg.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (*Client) shouldRetry(err error) bool {
	// Cheaper than falling through to isRetryable, which reaches the same
	// answer only after extra type assertions.
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.IsAuthenticationError() || apiErr.IsForbiddenError() {
			return false
		}
	}

	return isRetryable(err)
}
