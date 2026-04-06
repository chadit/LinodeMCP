package linode

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"
)

// APIError represents an error returned by the Linode API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
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

func isRetryable(err error) bool {
	if _, ok := errors.AsType[*RetryableError](err); ok {
		return true
	}

	if apiErr, ok := errors.AsType[*APIError](err); ok {
		return apiErr.IsRateLimitError() || apiErr.IsServerError()
	}

	return isNetworkError(err) || isTimeoutError(err)
}
