package tools_test

import (
	"encoding/json"
	"io"
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

func TestLinodeProfilePreferencesUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

		assert.Equal(t, "linode_profile_preferences_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be a write tool")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyPreferences, "schema should include preferences body")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "write tool must require confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "write tool should expose dry_run")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/profile/preferences", r.URL.Path, "request path should be /profile/preferences")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			body, readErr := io.ReadAll(r.Body)
			if !assert.NoError(t, readErr, "request body should be readable") {
				return
			}

			assert.JSONEq(t, `{"theme":"dark"}`, string(body), "request body should match tool input")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "dark", "response should include returned preferences")
	})

	t.Run("dry run previews without put", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyDryRun: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should not be an error result")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		would, wouldOK := body["would_execute"].(map[string]any)
		require.True(t, wouldOK, "dry run response should include would_execute")
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/profile/preferences", would["path"])
		previewBody, previewBodyOK := would["body"].(map[string]any)
		require.True(t, previewBodyOK, "dry run response should include the request body")
		assert.Equal(t, profilePreferenceValueDark, previewBody[profilePreferenceKeyTheme], "dry run body should include preference fields")
		assert.Equal(t, int32(0), calls.Load(), "dry run should not call the PUT endpoint")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "preferences update surfaces a side effect")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/profile/preferences", r.URL.Path, "request path should be /profile/preferences")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}, keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_profile_preferences_update")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false},
			{name: caseString, confirm: boolStringTrue},
			{name: caseNumericConfirm, confirm: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

				args := map[string]any{keyPreferences: map[string]any{profilePreferenceKeyTheme: profilePreferenceValueDark}}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or non-true confirm should be an error result")
				assertErrorContains(t, result, "Set confirm=true to proceed")
				assert.Equal(t, int32(0), calls.Load(), "confirm rejection should not call the client")
			})
		}
	})

	t.Run("preferences body required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			preferences any
		}{
			{name: caseMissing},
			{name: "empty object", preferences: map[string]any{}},
			{name: caseString, preferences: "theme=dark"},
			{name: caseNumericConfirm, preferences: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				defer srv.Close()

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeProfilePreferencesUpdateTool(cfg)

				args := map[string]any{keyConfirm: true}
				if testCase.name != caseMissing {
					args[keyPreferences] = testCase.preferences
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid preferences should be an error result")
				assertErrorContains(t, result, "preferences must be a non-empty object")
				assert.Equal(t, int32(0), calls.Load(), "validation rejection should not call the client")
			})
		}
	})
}
