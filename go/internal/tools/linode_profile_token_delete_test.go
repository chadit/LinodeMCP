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

const (
	keyProfileTokenID             = "token_id"
	profileTokenDeleteConfirmText = "confirm=true"
	profileTokenIDIntegerError    = "token_id must be an integer"
)

func TestLinodeProfileTokenDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

		assert.Equal(t, "linode_profile_token_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyProfileTokenID, "schema should include token_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, keyProfileTokenID, "token_id must be required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token id")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			assert.Equal(t, http.NoBody, r.Body, "request should not include a body")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := profileTokenTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyConfirm: true}))

		require.NoError(t, err, "handler should not return a Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "success should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Profile token revoked successfully", "response should include success message")
	})

	t.Run("dry run previews delete without client call", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := profileTokenTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyDryRun: true}))

		require.NoError(t, err, "handler should not return a Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should not be an error result")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		would, ok := body["would_execute"].(map[string]any)
		require.True(t, ok, "dry run response should include would_execute")
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/profile/tokens/12345", would["path"])
		assert.Equal(t, int32(0), requestCount.Load(), "dry run should not call the DELETE endpoint")
	})

	t.Run("confirm guard rejects before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			set     bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, set: true},
			{name: caseString, confirm: boolStringTrue, set: true},
			{name: caseNumber, confirm: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var requestCount atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requestCount.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer srv.Close()

				cfg := profileTokenTestConfig(srv.URL)
				_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

				args := map[string]any{keyProfileTokenID: 12345.0}
				if testCase.set {
					args[keyConfirm] = testCase.confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool result")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assert.Equal(t, int32(0), requestCount.Load(), "client must not be called when confirm is invalid")
				assertErrorContains(t, result, profileTokenDeleteConfirmText)
			})
		}
	})

	t.Run("invalid token_id rejects before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}, want: "token_id is required"},
			{name: caseZero, args: map[string]any{keyProfileTokenID: 0, keyConfirm: true}, want: "token_id must be an integer greater than or equal to 1"},
			{name: caseString, args: map[string]any{keyProfileTokenID: "12345", keyConfirm: true}, want: profileTokenIDIntegerError},
			{name: caseFractionalServiceID, args: map[string]any{keyProfileTokenID: 12345.5, keyConfirm: true}, want: profileTokenIDIntegerError},
			{name: caseSlashServiceID, args: map[string]any{keyProfileTokenID: placementGroupSlashID, keyConfirm: true}, want: profileTokenIDIntegerError},
			{name: caseQueryServiceID, args: map[string]any{keyProfileTokenID: "12?token=1", keyConfirm: true}, want: profileTokenIDIntegerError},
			{name: caseTraversalServiceID, args: map[string]any{keyProfileTokenID: pathTraversalValue, keyConfirm: true}, want: profileTokenIDIntegerError},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var requestCount atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requestCount.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer srv.Close()

				cfg := profileTokenTestConfig(srv.URL)
				_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool result")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid token_id should be an error result")
				assert.Equal(t, int32(0), requestCount.Load(), "client must not be called when token_id is invalid")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token id")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := profileTokenTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeProfileTokenDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyProfileTokenID: 12345.0, keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to delete linode_profile_token_delete")
		assertErrorContains(t, result, errForbidden)
	})
}
