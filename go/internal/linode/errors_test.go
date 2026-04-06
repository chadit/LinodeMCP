package linode_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// TestAPIErrorMessage verifies that APIError produces clear error strings
// with and without the Field property set.
//
// Workflow:
//  1. **Setup**: Create APIError instances with various field combinations
//  2. **Execute**: Call Error() on each
//  3. **Verify**: Check formatted string matches expected output
//
// Expected Behavior:
//   - Errors with Field include it in the message
//   - Errors without Field omit the field portion
//
// Purpose: Ensures API error messages are informative for debugging.
func TestAPIErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		apiErr         *linode.APIError
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:        "with field",
			apiErr:      &linode.APIError{StatusCode: 400, Message: "bad value", Field: "label"},
			mustContain: []string{"field: label", "status 400", "bad value"},
		},
		{
			name:           "without field",
			apiErr:         &linode.APIError{StatusCode: 500, Message: "internal error"},
			mustContain:    []string{"status 500"},
			mustNotContain: []string{"field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := tt.apiErr.Error()
			for _, s := range tt.mustContain {
				assert.Contains(t, msg, s, "error message should contain %q", s)
			}

			for _, s := range tt.mustNotContain {
				assert.NotContains(t, msg, s, "error message should not contain %q", s)
			}
		})
	}
}

// TestAPIErrorStatusChecks verifies that status-code classification methods
// on APIError correctly identify authentication, rate-limit, forbidden,
// and server error responses.
//
// Workflow:
//  1. **Setup**: Create APIError instances with various HTTP status codes
//  2. **Execute**: Call each classification method
//  3. **Verify**: Only the expected classification returns true
//
// Expected Behavior:
//   - 401 is an authentication error only
//   - 403 is a forbidden error only
//   - 429 is a rate-limit error only
//   - 500 and 503 are server errors only
//   - 404 matches none of the above
//
// Purpose: Confirms status code categorization drives correct retry and
// error-handling decisions.
func TestAPIErrorStatusChecks(t *testing.T) {
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
			assert.Equal(t, tt.isAuth, err.IsAuthenticationError(), "IsAuthenticationError mismatch for status %d", tt.statusCode)
			assert.Equal(t, tt.isRateLimit, err.IsRateLimitError(), "IsRateLimitError mismatch for status %d", tt.statusCode)
			assert.Equal(t, tt.isForbidden, err.IsForbiddenError(), "IsForbiddenError mismatch for status %d", tt.statusCode)
			assert.Equal(t, tt.isServerError, err.IsServerError(), "IsServerError mismatch for status %d", tt.statusCode)
		})
	}
}

// TestNetworkErrorErrorAndUnwrap verifies that NetworkError formats its
// message with the operation name and underlying error, and that Unwrap
// returns the original cause.
func TestNetworkErrorErrorAndUnwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("connection refused")
	err := &linode.NetworkError{Operation: "GetProfile", Err: inner}

	require.ErrorContains(t, err, "GetProfile", "error message should include the operation name")
	require.ErrorContains(t, err, "connection refused", "error message should include the underlying cause")
	assert.Equal(t, inner, err.Unwrap(), "Unwrap should return the original inner error")
}

// TestRetryableError verifies that RetryableError formats its message
// correctly (with and without RetryAfter) and that Unwrap exposes the
// underlying cause for errors.Is/As chains.
//
// Workflow:
//  1. **Setup**: Create RetryableError with and without RetryAfter
//  2. **Execute**: Call Error() and check Unwrap behavior
//  3. **Verify**: Messages include expected substrings, Unwrap returns inner
//
// Expected Behavior:
//   - With RetryAfter set, message contains "retry after" and the cause
//   - Without RetryAfter, message contains "retryable error" and the cause
//   - Unwrap returns the original inner error via errors.Is
//
// Purpose: Confirms retryable error formatting and unwrap chain integrity.
func TestRetryableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            *linode.RetryableError
		mustContain    []string
		mustNotContain []string
		unwrapTarget   error
	}{
		{
			name:        "with retry delay",
			err:         &linode.RetryableError{Err: errors.New("server busy"), RetryAfter: 5 * time.Second},
			mustContain: []string{"retry after", "server busy"},
		},
		{
			name:           "without retry delay",
			err:            &linode.RetryableError{Err: errors.New("server busy")},
			mustContain:    []string{"retryable error", "server busy"},
			mustNotContain: []string{"retry after"},
		},
		{
			name:         "unwrap returns inner",
			err:          &linode.RetryableError{Err: errors.New("inner")},
			unwrapTarget: errors.New("inner"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := tt.err.Error()
			for _, s := range tt.mustContain {
				assert.Contains(t, msg, s, "error message should contain %q", s)
			}

			for _, s := range tt.mustNotContain {
				assert.NotContains(t, msg, s, "error message should not contain %q", s)
			}

			if tt.unwrapTarget != nil {
				assert.ErrorIs(t, tt.err, tt.err.Err, "Unwrap should expose the inner error")
			}
		})
	}
}
