package linode_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Ensures API error messages are informative for debugging.
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
			apiErr:      &linode.APIError{StatusCode: 400, Message: "bad value", Field: keyLabel},
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
				if !strings.Contains(msg, s) {
					t.Errorf("msg does not contain %v", s)
				}
			}

			for _, s := range tt.mustNotContain {
				if strings.Contains(msg, s) {
					t.Errorf("msg should not contain %v", s)
				}
			}
		})
	}
}

// Confirms status code categorization drives correct retry and error-handling decisions.
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
			if err.IsAuthenticationError() != tt.isAuth {
				t.Errorf("err.IsAuthenticationError() = %v, want %v", err.IsAuthenticationError(), tt.isAuth)
			}

			if err.IsRateLimitError() != tt.isRateLimit {
				t.Errorf("err.IsRateLimitError() = %v, want %v", err.IsRateLimitError(), tt.isRateLimit)
			}

			if err.IsForbiddenError() != tt.isForbidden {
				t.Errorf("err.IsForbiddenError() = %v, want %v", err.IsForbiddenError(), tt.isForbidden)
			}

			if err.IsServerError() != tt.isServerError {
				t.Errorf("err.IsServerError() = %v, want %v", err.IsServerError(), tt.isServerError)
			}
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

	if got := err.Error(); got != "network error during GetProfile: connection refused" {
		t.Errorf("err.Error() = %q, want %q", got, "network error during GetProfile: connection refused")
	}

	if !reflect.DeepEqual(err.Unwrap(), inner) {
		t.Errorf("err.Unwrap() = %v, want %v", err.Unwrap(), inner)
	}
}

// Confirms retryable error formatting and unwrap chain integrity.
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
				if !strings.Contains(msg, s) {
					t.Errorf("msg does not contain %v", s)
				}
			}

			for _, s := range tt.mustNotContain {
				if strings.Contains(msg, s) {
					t.Errorf("msg should not contain %v", s)
				}
			}

			if tt.unwrapTarget != nil {
				if err := tt.err; !errors.Is(err, tt.err.Err) {
					t.Errorf("error = %v, want %v", err, tt.err.Err)
				}
			}
		})
	}
}
