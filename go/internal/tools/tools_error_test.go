package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestToolHandlersAPIErrorResponses verifies that tool handlers gracefully
// handle various HTTP error status codes from the Linode API by returning
// structured error results instead of Go errors.
//
// Workflow:
//  1. **ListInstances**: Test 500, 401, 403, 429 responses
//  2. **GetInstance**: Test 500, 401, 403, 429 responses
//  3. **CreateInstance**: Test 500, 401, 403, 429 responses
//  4. **ListDomainRecords**: Test 500, 401, 403, 429 responses
//
// Expected Behavior:
//   - All tools return result.IsError=true for API errors
//   - Error message from the API is included in the result
//   - No Go-level errors are returned (errors are result-level)
//
// Purpose: Confirm that API failures are surfaced as user-visible error
// results across multiple tool handlers.
func TestToolHandlersAPIErrorResponses(t *testing.T) {
	t.Parallel()

	type errorCase struct {
		statusCode   int
		errorMessage string
		label        string
	}

	errorCases := []errorCase{
		{statusCode: http.StatusInternalServerError, errorMessage: "server error", label: "500_server_error"},
		{statusCode: http.StatusUnauthorized, errorMessage: "invalid token", label: "401_unauthorized"},
		{statusCode: http.StatusForbidden, errorMessage: "forbidden", label: "403_forbidden"},
		{statusCode: http.StatusTooManyRequests, errorMessage: "rate limit", label: "429_rate_limit"},
	}

	t.Run("ListInstances", func(t *testing.T) {
		t.Parallel()

		for _, errCase := range errorCases {
			t.Run(errCase.label, func(t *testing.T) {
				t.Parallel()

				srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
				defer srv.Close()

				cfg := newTestConfig(srv.URL)
				_, handler := tools.NewLinodeInstancesTool(cfg)

				req := createRequestWithArgs(t, map[string]any{})
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool errors are returned as error results, not Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "should return an error result for status %d", errCase.statusCode)
				assertErrorContains(t, result, errCase.errorMessage)
			})
		}
	})

	t.Run("GetInstance", func(t *testing.T) {
		t.Parallel()

		for _, errCase := range errorCases {
			t.Run(errCase.label, func(t *testing.T) {
				t.Parallel()

				srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
				defer srv.Close()

				cfg := newTestConfig(srv.URL)
				_, handler := tools.NewLinodeInstanceGetTool(cfg)

				req := createRequestWithArgs(t, map[string]any{"instance_id": "123"})
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool errors are returned as error results, not Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "should return an error result for status %d", errCase.statusCode)
				assertErrorContains(t, result, errCase.errorMessage)
			})
		}
	})

	t.Run("CreateInstance", func(t *testing.T) {
		t.Parallel()

		for _, errCase := range errorCases {
			t.Run(errCase.label, func(t *testing.T) {
				t.Parallel()

				srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
				defer srv.Close()

				cfg := newTestConfig(srv.URL)
				_, handler := tools.NewLinodeInstanceCreateTool(cfg)

				req := createRequestWithArgs(t, map[string]any{
					"confirm":   true,
					"region":    "us-east",
					"type":      "g6-nanode-1",
					"image":     "linode/ubuntu22.04",
					"label":     "test",
					"root_pass": "Str0ngP@ssw0rd!",
				})
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool errors are returned as error results, not Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "should return an error result for status %d", errCase.statusCode)
				assertErrorContains(t, result, errCase.errorMessage)
			})
		}
	})

	t.Run("ListDomainRecords", func(t *testing.T) {
		t.Parallel()

		for _, errCase := range errorCases {
			t.Run(errCase.label, func(t *testing.T) {
				t.Parallel()

				srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
				defer srv.Close()

				cfg := newTestConfig(srv.URL)
				_, handler := tools.NewLinodeDomainRecordsListTool(cfg)

				req := createRequestWithArgs(t, map[string]any{"domain_id": "123"})
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "tool errors are returned as error results, not Go errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "should return an error result for status %d", errCase.statusCode)
				assertErrorContains(t, result, errCase.errorMessage)
			})
		}
	})
}

// TestToolHandlersMalformedJSONErrorResponse verifies that tool handlers
// gracefully handle malformed JSON in API error responses by returning a
// structured error result with a generic error message.
func TestToolHandlersMalformedJSONErrorResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	_, handler := tools.NewLinodeInstancesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err, "tool errors are returned as error results, not Go errors")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.IsError, "should return an error result for malformed JSON response")
	assertErrorContains(t, result, "internal server error")
}

// newErrorServer creates an httptest server that returns a Linode API error response.
func newErrorServer(t *testing.T, statusCode int, errorMessage string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": errorMessage}},
		}), "test server should encode error response")
	}))
}

// newTestConfig builds a config pointing at the given test server URL.
func newTestConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: apiURL, Token: "test-token"},
			},
		},
	}
}
