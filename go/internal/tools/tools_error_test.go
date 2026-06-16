package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// Confirm that API failures are surfaced as user-visible error results across multiple tool handlers.
func TestToolHandlersAPIErrorResponsesListInstances(t *testing.T) {
	type errorCase struct {
		statusCode   int
		errorMessage string
		label        string
	}

	errorCases := []errorCase{
		{statusCode: http.StatusInternalServerError, errorMessage: tcServerErrorMsg, label: tcServerError},
		{statusCode: http.StatusUnauthorized, errorMessage: tcInvalidTokenMsg, label: tcUnauthorized},
		{statusCode: http.StatusForbidden, errorMessage: errForbidden, label: tcForbidden},
		{statusCode: http.StatusTooManyRequests, errorMessage: tcRateLimitMsg, label: tcRateLimit},
	}

	t.Parallel()

	for _, errCase := range errorCases {
		t.Run(errCase.label, func(t *testing.T) {
			t.Parallel()

			srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeInstanceListTool(cfg)

			req := createRequestWithArgs(t, map[string]any{})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errCase.errorMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, errCase.errorMessage)
			}
		})
	}
}

func TestToolHandlersAPIErrorResponsesGetInstance(t *testing.T) {
	type errorCase struct {
		statusCode   int
		errorMessage string
		label        string
	}

	errorCases := []errorCase{
		{statusCode: http.StatusInternalServerError, errorMessage: tcServerErrorMsg, label: tcServerError},
		{statusCode: http.StatusUnauthorized, errorMessage: tcInvalidTokenMsg, label: tcUnauthorized},
		{statusCode: http.StatusForbidden, errorMessage: errForbidden, label: tcForbidden},
		{statusCode: http.StatusTooManyRequests, errorMessage: tcRateLimitMsg, label: tcRateLimit},
	}

	t.Parallel()

	for _, errCase := range errorCases {
		t.Run(errCase.label, func(t *testing.T) {
			t.Parallel()

			srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

			req := createRequestWithArgs(t, map[string]any{keyInstanceID: "123"})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errCase.errorMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, errCase.errorMessage)
			}
		})
	}
}

func TestToolHandlersAPIErrorResponsesCreateInstance(t *testing.T) {
	type errorCase struct {
		statusCode   int
		errorMessage string
		label        string
	}

	errorCases := []errorCase{
		{statusCode: http.StatusInternalServerError, errorMessage: tcServerErrorMsg, label: tcServerError},
		{statusCode: http.StatusUnauthorized, errorMessage: tcInvalidTokenMsg, label: tcUnauthorized},
		{statusCode: http.StatusForbidden, errorMessage: errForbidden, label: tcForbidden},
		{statusCode: http.StatusTooManyRequests, errorMessage: tcRateLimitMsg, label: tcRateLimit},
	}

	t.Parallel()

	for _, errCase := range errorCases {
		t.Run(errCase.label, func(t *testing.T) {
			t.Parallel()

			srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeInstanceCreateTool(cfg)

			req := createRequestWithArgs(t, map[string]any{
				keyConfirm:    true,
				keyRegion:     regionUSEast,
				keyType:       typeG6Nanode1,
				keyImage:      imageIDUbuntu2204,
				keyLabel:      "test",
				keyRootPass:   rootPassStrong,
				keyFirewallID: 12345,
			})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errCase.errorMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, errCase.errorMessage)
			}
		})
	}
}

func TestToolHandlersAPIErrorResponsesListDomainRecords(t *testing.T) {
	type errorCase struct {
		statusCode   int
		errorMessage string
		label        string
	}

	errorCases := []errorCase{
		{statusCode: http.StatusInternalServerError, errorMessage: tcServerErrorMsg, label: tcServerError},
		{statusCode: http.StatusUnauthorized, errorMessage: tcInvalidTokenMsg, label: tcUnauthorized},
		{statusCode: http.StatusForbidden, errorMessage: errForbidden, label: tcForbidden},
		{statusCode: http.StatusTooManyRequests, errorMessage: tcRateLimitMsg, label: tcRateLimit},
	}

	t.Parallel()

	for _, errCase := range errorCases {
		t.Run(errCase.label, func(t *testing.T) {
			t.Parallel()

			srv := newErrorServer(t, errCase.statusCode, errCase.errorMessage)
			defer srv.Close()

			cfg := newTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeDomainRecordListTool(cfg)

			req := createRequestWithArgs(t, map[string]any{keyDomainID: "123"})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errCase.errorMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, errCase.errorMessage)
			}
		})
	}
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
	_, _, handler := tools.NewLinodeInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "internal server error") {
		t.Errorf("error text %q does not contain %q", text.Text, "internal server error")
	}
}

// newErrorServer creates an httptest server that returns a Linode API error response.
func newErrorServer(t *testing.T, statusCode int, errorMessage string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"reason": errorMessage}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
}

// newTestConfig builds a config pointing at the given test server URL.
func newTestConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}
