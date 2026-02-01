package linode_test

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestAPIError_Error_WithField(t *testing.T) {
	t.Parallel()

	err := &linode.APIError{StatusCode: 400, Message: "bad value", Field: "label"}
	assert.Contains(t, err.Error(), "field: label")
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "bad value")
}

func TestAPIError_Error_WithoutField(t *testing.T) {
	t.Parallel()

	err := &linode.APIError{StatusCode: 500, Message: "internal error"}
	assert.NotContains(t, err.Error(), "field")
	assert.Contains(t, err.Error(), "status 500")
}

func TestAPIError_StatusChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		statusCode    int
		isAuth        bool
		isRateLimit   bool
		isForbidden   bool
		isServerError bool
	}{
		{"unauthorized", 401, true, false, false, false},
		{"forbidden", 403, false, false, true, false},
		{"rate limit", 429, false, true, false, false},
		{"server error 500", 500, false, false, false, true},
		{"server error 503", 503, false, false, false, true},
		{"client error 404", 404, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &linode.APIError{StatusCode: tt.statusCode, Message: "test"}
			assert.Equal(t, tt.isAuth, err.IsAuthenticationError(), "IsAuthenticationError mismatch.")
			assert.Equal(t, tt.isRateLimit, err.IsRateLimitError(), "IsRateLimitError mismatch.")
			assert.Equal(t, tt.isForbidden, err.IsForbiddenError(), "IsForbiddenError mismatch.")
			assert.Equal(t, tt.isServerError, err.IsServerError(), "IsServerError mismatch.")
		})
	}
}

func TestNetworkError_ErrorAndUnwrap(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("connection refused")
	err := &linode.NetworkError{Operation: "GetProfile", Err: inner}

	assert.Contains(t, err.Error(), "GetProfile")
	assert.Contains(t, err.Error(), "connection refused")
	assert.Equal(t, inner, err.Unwrap())
}

func TestIsNetworkError(t *testing.T) {
	t.Parallel()

	assert.True(t, linode.IsNetworkError(&linode.NetworkError{Operation: "test", Err: fmt.Errorf("fail")}))
	assert.True(t, linode.IsNetworkError(&url.Error{Op: "Get", URL: "http://x", Err: fmt.Errorf("fail")}))
	assert.False(t, linode.IsNetworkError(fmt.Errorf("random error")))
}

type mockNetError struct {
	timeout bool
}

func (m *mockNetError) Error() string   { return "mock net error" }
func (m *mockNetError) Timeout() bool   { return m.timeout }
func (m *mockNetError) Temporary() bool { return false }

// Ensure mockNetError satisfies net.Error at compile time.
var _ net.Error = (*mockNetError)(nil)

func TestIsTimeoutError(t *testing.T) {
	t.Parallel()

	assert.True(t, linode.IsTimeoutError(&mockNetError{timeout: true}))
	assert.False(t, linode.IsTimeoutError(&mockNetError{timeout: false}))
	assert.False(t, linode.IsTimeoutError(fmt.Errorf("not a timeout")))
}

func TestIsTimeoutError_WrappedInURLError(t *testing.T) {
	t.Parallel()

	urlErr := &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: &mockNetError{timeout: true},
	}
	assert.True(t, linode.IsTimeoutError(urlErr))
}

func TestRetryableError_ErrorMessage(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("server busy")

	withDelay := &linode.RetryableError{Err: inner, RetryAfter: 5 * time.Second}
	assert.Contains(t, withDelay.Error(), "retry after")
	assert.Contains(t, withDelay.Error(), "server busy")

	withoutDelay := &linode.RetryableError{Err: inner}
	assert.NotContains(t, withoutDelay.Error(), "retry after")
	assert.Contains(t, withoutDelay.Error(), "retryable error")
}

func TestRetryableError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("inner")
	err := &linode.RetryableError{Err: inner}
	assert.True(t, errors.Is(err, inner))
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"retryable error type", &linode.RetryableError{Err: fmt.Errorf("x")}, true},
		{"rate limit API error", &linode.APIError{StatusCode: 429, Message: "rate"}, true},
		{"server error API error", &linode.APIError{StatusCode: 500, Message: "server"}, true},
		{"auth API error not retryable", &linode.APIError{StatusCode: 401, Message: "auth"}, false},
		{"404 API error not retryable", &linode.APIError{StatusCode: 404, Message: "not found"}, false},
		{"network error", &linode.NetworkError{Operation: "test", Err: fmt.Errorf("conn")}, true},
		{"timeout error", &mockNetError{timeout: true}, true},
		{"plain error not retryable", fmt.Errorf("plain"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.retryable, linode.IsRetryable(tt.err), "IsRetryable mismatch for %s.", tt.name)
		})
	}
}
