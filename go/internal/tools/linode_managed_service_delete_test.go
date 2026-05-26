package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const managedServiceDeleteToolName = "linode_managed_service_delete"

func TestLinodeManagedServiceDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedServiceDeleteTool(cfg)

		assert.Equal(t, managedServiceDeleteToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "managed service delete should be destructive")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyManagedServiceID, "schema should include service_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyManagedServiceID, "service_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissingConfirm, set: false},
			{name: caseRequiresConfirm, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

				args := map[string]any{keyManagedServiceID: managedServiceToolIDValue}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
			})
		}
	})

	t.Run("invalid service id rejected before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingServiceID, args: map[string]any{keyConfirm: true}, wantMessage: "service_id is required"},
			{name: caseZeroServiceID, args: map[string]any{keyManagedServiceID: 0, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: "negative service id", args: map[string]any{keyManagedServiceID: -1, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: "string service id", args: map[string]any{keyManagedServiceID: "9944", keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: "fractional service id", args: map[string]any{keyManagedServiceID: 9944.5, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: "oversized service id", args: map[string]any{keyManagedServiceID: managedServiceOversizedID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: caseSlashServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceSlashID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: caseQueryServiceID, args: map[string]any{keyManagedServiceID: invalidManagedServiceQueryID, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
			{name: caseTraversalServiceID, args: map[string]any{keyManagedServiceID: pathTraversalValue, keyConfirm: true}, wantMessage: errManagedServiceIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid service ID should be a tool error")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls, "validation failure must happen before client call")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted successfully", "response should confirm deletion")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, managedServiceToolPathValue, r.URL.Path, "request path should include service ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeManagedServiceDeleteTool(managedServiceConfig(srv.URL))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyManagedServiceID: managedServiceToolIDValue, keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to delete linode_managed_service_delete")
		assertErrorContains(t, result, errForbidden)
	})
}
