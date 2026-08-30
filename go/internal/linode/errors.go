package linode

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

var (
	errResponseBodyNotJSONArray  = errors.New("response body is not a JSON array")
	errResponseBodyNotJSONObject = errors.New("response body is not a JSON object")
)

// ErrShapeMismatch reports a collection fetched through the wrong primitive for
// its declared envelope. Answering anyway would drop the cursor a marker-paged
// caller resumes from, or invent one for a page that carries none.
var ErrShapeMismatch = errors.New("list envelope does not match the primitive it was fetched through")

// ErrWriteResponseNotObject rejects a mutation response that is not a JSON
// object. Callers prefix the call it belongs to, so the sentence a client sees
// names which mutation answered badly rather than which decoder complained;
// the Python client emits the same wording so one fixture covers both.
var ErrWriteResponseNotObject = errors.New("response must be a JSON object")

// ErrStateMemberMissing rejects a state read whose answer does not carry the
// member the contract says the resource sits under. Decoding the envelope
// instead would report an empty resource as the state a delete is about to
// remove, which a plan would then hash and a preview would show.
var ErrStateMemberMissing = errors.New("state response carries no member")

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
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	// Method is the HTTP method of the request that produced this error, not
	// part of the API payload. It gates 5xx retry: a server error on a POST may
	// have been applied before the error surfaced, so it must not be replayed.
	Method     string        `json:"-"`
	StatusCode int           `json:"status_code"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
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
	Err       error
	Operation string
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error during %s: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error { return e.Err }

// ArgumentError reports a request that could not be built: a tool the proto
// contract declares no route for, or path values that do not fill its
// template. Separate from NetworkError because nothing was sent, so a retry
// cannot help.
type ArgumentError struct {
	Err       error
	Operation string
}

func (e *ArgumentError) Error() string {
	return fmt.Sprintf("invalid arguments for %s: %v", e.Operation, e.Err)
}

func (e *ArgumentError) Unwrap() error { return e.Err }

// wrapRequestError classifies a client method's failure. A route the contract
// could not resolve or fill never reached the network; everything else keeps
// the network class the retry layer already reads. Call sites that resolve a
// route through the contract use this instead of building a NetworkError.
func wrapRequestError(operation string, err error) error {
	if linoderoute.IsContractError(err) {
		return &ArgumentError{Operation: operation, Err: err}
	}

	return &NetworkError{Operation: operation, Err: err}
}

func isNetworkError(err error) bool {
	if _, ok := errors.AsType[*NetworkError](err); ok {
		// A route or argument failure describes a request that was never sent,
		// so the retry loop must not read it as a replayable transport failure
		// even when a call site put it in this class.
		return !linoderoute.IsContractError(err)
	}

	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	_, ok := errors.AsType[*url.Error](err)

	return ok
}

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
// method gates retry: a transport failure on a POST may have reached and been
// processed by the server before the error surfaced locally, so replaying it
// could duplicate the side effect.
type requestError struct {
	Err    error
	Method string
}

func (e *requestError) Error() string { return "request failed: " + e.Err.Error() }

func (e *requestError) Unwrap() error { return e.Err }

// isIdempotentMethod reports whether replaying a failed request of this HTTP
// method is safe. POST, PATCH, and any unknown or empty method count as unsafe
// because the failed request may already have been applied.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

// isRetryable reports whether err is a transient failure worth retrying. A 429
// was rejected before processing, so it is always safe to replay; a 5xx or a
// transport failure may have been applied server-side, so those are retried
// only when the underlying request was idempotent.
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
