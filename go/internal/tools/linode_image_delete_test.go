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

func TestLinodeImageDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageDeleteTool(cfg)

		assert.Equal(t, "linode_image_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyImageID, "schema should include image_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "destructive tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyImageID, "image_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyImageID: privateImage12345Fixture}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyImageID: privateImage12345Fixture, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyImageID: privateImage12345Fixture, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyImageID: privateImage12345Fixture, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingImageID, args: map[string]any{keyConfirm: true}, wantContains: errImageIDNonEmpty},
		{name: "blank image id", args: map[string]any{keyImageID: blankString, keyConfirm: true}, wantContains: errImageIDNonEmpty},
		{name: caseQueryImageID, args: map[string]any{keyImageID: "private/123?query", keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "fragment image id", args: map[string]any{keyImageID: "private/123#frag", keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: caseTraversalImageID, args: map[string]any{keyImageID: privateImageTraversalFixture, keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "separator-only image id", args: map[string]any{keyImageID: "/private/123", keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "public image id", args: map[string]any{keyImageID: imageIDUbuntu2204, keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "non-numeric private image id", args: map[string]any{keyImageID: "private/not-a-number", keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "extra segment image id", args: map[string]any{keyImageID: "private/123/456", keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "zero private image id", args: map[string]any{keyImageID: privateImageZeroFixture, keyConfirm: true}, wantContains: errImageIDPathFragment},
		{name: "signed private image id", args: map[string]any{keyImageID: "private/+123", keyConfirm: true}, wantContains: errImageIDPathFragment},
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
			_, _, handler := tools.NewLinodeImageDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid delete request should be an error result")
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
			assert.Equal(t, "/images/private%2F12345", r.URL.EscapedPath(), "request path should include encoded image ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture, keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted successfully", "response should include success message")
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
		_, _, handler := tools.NewLinodeImageDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture, keyConfirm: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "client failure should be an error result")
		assertErrorContains(t, result, "linode_image_delete failed")
	})
}

// Dry-run coverage for image delete.
func TestLinodeImageDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					keyBetaID: "private/12345", keyLabel: "my-image",
				}))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeImageDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage12345Fixture,
			keyDryRun:  true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_image_delete", body["tool"])
		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/images/private/12345", would["path"])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("still validates image_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))

		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errImageIDNonEmpty)
	})
}
