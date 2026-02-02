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
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
}

// Error returns a formatted string describing the API error.
func (e *APIError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("Linode API error (status %d): %s (field: %s)", e.StatusCode, e.Message, e.Field)
	}

	return fmt.Sprintf("Linode API error (status %d): %s", e.StatusCode, e.Message)
}

const (
	httpUnauthorizedErr = 401
	httpForbiddenErr    = 403
	httpTooManyReqsErr  = 429
	httpServerErrorMin  = 500
	httpServerErrorMax  = 600
)

// IsAuthenticationError returns true if the status code is 401 Unauthorized.
func (e *APIError) IsAuthenticationError() bool { return e.StatusCode == httpUnauthorizedErr }

// IsRateLimitError returns true if the status code is 429 Too Many Requests.
func (e *APIError) IsRateLimitError() bool { return e.StatusCode == httpTooManyReqsErr }

// IsForbiddenError returns true if the status code is 403 Forbidden.
func (e *APIError) IsForbiddenError() bool { return e.StatusCode == httpForbiddenErr }

// IsServerError returns true if the status code indicates a server error (5xx).
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= httpServerErrorMin && e.StatusCode < httpServerErrorMax
}

// NetworkError represents a network-related error.
type NetworkError struct {
	Operation string
	Err       error
}

// Error returns a formatted string describing the network error.
func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error.
func (e *NetworkError) Unwrap() error { return e.Err }

// IsNetworkError returns true if the error is a network-related error.
func IsNetworkError(err error) bool {
	var networkErr *NetworkError
	if errors.As(err, &networkErr) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error

	return errors.As(err, &urlErr)
}

// IsTimeoutError returns true if the error is a timeout error.
func IsTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var innerNetErr net.Error
		if errors.As(urlErr.Err, &innerNetErr) {
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

// Error returns a formatted string describing the retryable error.
func (e *RetryableError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("retryable error (retry after %v): %v", e.RetryAfter, e.Err)
	}

	return fmt.Sprintf("retryable error: %v", e.Err)
}

// Unwrap returns the underlying error.
func (e *RetryableError) Unwrap() error { return e.Err }

// IsRetryable returns true if the error is eligible for retry.
func IsRetryable(err error) bool {
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.IsRateLimitError() || apiErr.IsServerError()
	}

	return IsNetworkError(err) || IsTimeoutError(err)
}
