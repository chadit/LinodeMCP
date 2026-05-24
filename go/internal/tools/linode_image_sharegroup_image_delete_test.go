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

func TestLinodeImageShareGroupImageDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

		assert.Equal(t, "linode_image_sharegroup_image_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		assert.Contains(t, tool.InputSchema.Properties, keyImageID, "schema should include image_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyShareGroupID, "sharegroup_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyImageID, "image_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing sharegroup id", args: map[string]any{keyImageID: 5678, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "zero sharegroup id", args: map[string]any{keyShareGroupID: 0, keyImageID: 5678, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "slash sharegroup id", args: map[string]any{keyShareGroupID: pathSeparatorValue, keyImageID: 5678, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "query sharegroup id", args: map[string]any{keyShareGroupID: "1?2", keyImageID: 5678, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "traversal sharegroup id", args: map[string]any{keyShareGroupID: pathTraversalValue, keyImageID: 5678, keyConfirm: true}, wantContains: errShareGroupIDPositive},
		{name: "missing image id", args: map[string]any{keyShareGroupID: 1234, keyConfirm: true}, wantContains: errImageIDPositive},
		{name: "zero image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: 0, keyConfirm: true}, wantContains: errImageIDPositive},
		{name: "slash image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: pathSeparatorValue, keyConfirm: true}, wantContains: errImageIDPositive},
		{name: "query image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: "5?6", keyConfirm: true}, wantContains: errImageIDPositive},
		{name: "traversal image id", args: map[string]any{keyShareGroupID: 1234, keyImageID: pathTraversalValue, keyConfirm: true}, wantContains: errImageIDPositive},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid image delete request should be an error result")
			assertErrorContains(t, result, tt.wantContains)
			assert.False(t, called.Load(), "validation should reject before client call")
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/images/sharegroups/1234/images/5678", r.URL.Path, "request path should include share group and image IDs")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed", "response should include success message")
		assert.Equal(t, int32(1), requestCount.Load(), "delete should make one request")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageShareGroupImageDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 1234, keyImageID: 5678, keyConfirm: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "Failed to remove shared image 5678")
	})
}
