package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyTFACode             = "tfa_code"
	keyTFAConfirmExpiry    = "expiry"
	tfaConfirmExpiry       = "2026-01-01T00:00:00"
	tfaConfirmScratchToken = "setup-token"
)

func TestLinodeProfileTFAEnableConfirmTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

		assert.Equal(t, "linode_profile_tfa_enable_confirm", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should be an admin tool")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyTFACode, "tool must require a TFA code")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "security-state-changing tool must require confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "admin tool should expose dry_run")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, map[string]string{keyTFACode: "123456"}, body)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"scratch": tfaConfirmScratchToken, keyTFAConfirmExpiry: tfaConfirmExpiry}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		assertErrorContains(t, result, tfaConfirmScratchToken)
	})

	t.Run("dry run previews body without post", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyDryRun: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "dry run should not be an error result")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		dryRun, dryRunOK := body[keyDryRun].(bool)
		require.True(t, dryRunOK, "dry_run should be a boolean")
		assert.True(t, dryRun, "response should be a dry-run preview")

		would, wouldOK := body["would_execute"].(map[string]any)
		require.True(t, wouldOK, "dry run response should include would_execute")
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/profile/tfa-enable-confirm", would["path"])
		wouldBody, bodyOK := would["body"].(map[string]any)
		require.True(t, bodyOK, "dry run response should include request body")
		assert.Equal(t, "123456", wouldBody[keyTFACode])
		assert.Equal(t, int32(0), calls.Load(), "dry run should not call the POST endpoint")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "confirming 2FA surfaces a side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "enabled", "side effect should state 2FA is enabled")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyTFACode: "123456", keyConfirm: true})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to confirm linode_profile_tfa_enable_confirm")
		assertErrorContains(t, result, errForbidden)
	})

	t.Run("tfa code required before confirm and client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			code any
		}{
			{name: caseMissing},
			{name: caseEmpty, code: ""},
			{name: caseString, code: 123456},
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
				_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

				args := map[string]any{keyConfirm: true}
				if testCase.name != caseMissing {
					args[keyTFACode] = testCase.code
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid tfa_code should be an error result")
				assertErrorContains(t, result, "tfa_code must be a non-empty string")
				assert.Equal(t, int32(0), calls.Load(), "TFA code validation should run before client call")
			})
		}
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
				_, _, handler := tools.NewLinodeProfileTFAEnableConfirmTool(cfg)

				args := map[string]any{keyTFACode: "123456"}
				if testCase.name != caseMissing {
					args[keyConfirm] = testCase.confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
				assert.Equal(t, int32(0), calls.Load(), "confirm gate should run before client call")
			})
		}
	})
}
