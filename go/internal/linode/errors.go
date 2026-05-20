package linode

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open and rejecting
// requests. Callers can check this sentinel to distinguish "we never tried"
// from "we tried and the upstream failed".
var ErrCircuitOpen = errors.New("circuit breaker open")

// ErrRateLimitWaitCanceled is returned when a caller's context is canceled
// while a goroutine is blocked waiting for a token. The breaker shouldn't
// count this; it's a caller-side decision, not an upstream-health signal.
var ErrRateLimitWaitCanceled = errors.New("rate limit wait canceled")

// APIError represents an error returned by the Linode API.
// RetryAfter carries the server's Retry-After hint when present so the retry
// loop can honor it instead of computing its own backoff.
type APIError struct {
	StatusCode int           `json:"status_code"`
	Message    string        `json:"message"`
	Field      string        `json:"field,omitempty"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	// Method is the HTTP method of the request that produced this error. It
	// drives retry safety for 5xx responses: a server error on a
	// non-idempotent request (POST) may have been applied before the error
	// surfaced, so it must not be replayed. Not part of the API payload.
	Method string `json:"-"`
}

func (e *APIError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("Linode API error (status %d): %s (field: %s)", e.StatusCode, e.Message, e.Field)
	}

	return fmt.Sprintf("Linode API error (status %d): %s", e.StatusCode, e.Message)
}

// IsAuthenticationError returns true if the status code is 401 Unauthorized.
func (e *APIError) IsAuthenticationError() bool { return e.StatusCode == httpUnauthorized }

// IsRateLimitError returns true if the status code is 429 Too Many Requests.
func (e *APIError) IsRateLimitError() bool { return e.StatusCode == httpTooManyReqs }

// IsForbiddenError returns true if the status code is 403 Forbidden.
func (e *APIError) IsForbiddenError() bool { return e.StatusCode == httpForbidden }

// IsServerError returns true if the status code indicates a server error (5xx).
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= httpServerError && e.StatusCode < httpServerErrorMax
}

// NetworkError represents a network-related error.
type NetworkError struct {
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error { return e.Err }

// isNetworkError returns true if the error is a network-related error.
func isNetworkError(err error) bool {
	if _, ok := errors.AsType[*NetworkError](err); ok {
		return true
	}

	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	_, ok := errors.AsType[*url.Error](err)

	return ok
}

// isTimeoutError returns true if the error is a timeout error.
func isTimeoutError(err error) bool {
	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		if innerNetErr, ok := errors.AsType[net.Error](urlErr.Err); ok {
			return innerNetErr.Timeout()
		}
	}

	return false
}

// RetryableError represents an error that can be retried.
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retryable error (retry after %v): %v", e.RetryAfter, e.Err)
	}

	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error { return e.Err }

// requestError wraps a transport-level failure (timeout, connection reset,
// DNS, refused) with the HTTP method of the request that produced it. The
// method drives retry safety: a transport failure on a non-idempotent request
// (POST) may have reached and been processed by the server before the error
// surfaced locally, so replaying it could duplicate the side effect.
type requestError struct {
	Method string
	Err    error
}

func (e *requestError) Error() string { return "request failed: " + e.Err.Error() }

func (e *requestError) Unwrap() error { return e.Err }

// isIdempotentMethod reports whether replaying a failed request of this HTTP
// method is safe. GET, HEAD, PUT, DELETE, and OPTIONS are idempotent by HTTP
// semantics: a retry converges to the same end state. POST (and PATCH) are
// not, so a failure that might already have been applied must not be retried.
// An unknown or empty method is treated as non-idempotent to stay on the safe
// side.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isRetryable reports whether err is a transient failure worth retrying.
// A rate-limit (429) is always safe to replay because the request was rejected
// before processing. A 5xx or a transport failure may have been applied
// server-side, so those are retried only when the underlying request was
// idempotent.
func isRetryable(err error) bool {
	if _, ok := errors.AsType[*RetryableError](err); ok {
		return true
	}

	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.IsRateLimitError() {
			return true
		}

		return apiErr.IsServerError() && isIdempotentMethod(apiErr.Method)
	}

	if isNetworkError(err) || isTimeoutError(err) {
		reqErr, ok := errors.AsType[*requestError](err)

		return ok && isIdempotentMethod(reqErr.Method)
	}

	return false
}
