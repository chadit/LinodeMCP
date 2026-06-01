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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	profileTokenIDFixture      = float64(12345)
	profileTokenLabel          = "ci-token"
	profileTokenUpdatedScopes  = "linodes:read_write"
	profileTokenIDParam        = "token_id"
	profileTokenScopesField    = "scopes"
	errProfileTokenIDPositive  = "token_id must be a positive integer"
	profileTokenQueryValue     = "12?x=1"
	profileTokenInvalidIDValue = "abc"
)

func TestLinodeProfileTokensTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileTokensTool(cfg)

		assert.Equal(t, "linode_profile_tokens", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "profile tokens should be read-only")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should expose optional page")
		assert.Contains(t, props, keyPageSize, "schema should expose optional page_size")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
				Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
				Page: 1, Pages: 1, Results: 1,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTokensTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyPage: 2.0, keyPageSize: 25.0}))

		require.NoError(t, err, "handler should not return a Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "success should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, profileTokenLabel, "response should include token label")
	})

	t.Run("invalid pagination", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeProfileTokensTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation failures are returned as error results")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be rejected before client call")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe pagination validation")
			})
		}
	})

	t.Run("upstream error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTokensTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))

		require.NoError(t, err, "upstream failures are returned as error results")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_profile_tokens")
	})
}

func TestLinodeProfileTokenUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokenUpdateTool(cfg)

	assert.Equal(t, "linode_profile_token_update", tool.Name, "tool name should match")
	assert.Equal(t, profiles.CapAdmin, capability, "profile token update should be admin-capable")
	require.NotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties

	assert.Contains(t, props, profileTokenIDParam, "schema should include token_id")
	assert.Contains(t, props, keyConfirm, "schema should include confirm")

	for _, field := range []string{keyExpiry, keyLabel, profileTokenScopesField} {
		assert.Contains(t, props, field, "schema should include documented body field")
	}

	assert.Contains(t, tool.InputSchema.Required, profileTokenIDParam, "token_id must be required")
	assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
}

func TestLinodeProfileTokenUpdateRequiresConfirm(t *testing.T) {
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

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))

			args := map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "confirm failures are returned as tool errors")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "confirm failure should be an error result")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}
}

func TestLinodeProfileTokenUpdateValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing token id", args: map[string]any{keyLabel: profileTokenLabel, keyConfirm: true}, want: errProfileTokenIDPositive},
		{name: "slash token id", args: map[string]any{profileTokenIDParam: placementGroupSlashID, keyLabel: profileTokenLabel, keyConfirm: true}, want: errProfileTokenIDPositive},
		{name: "query token id", args: map[string]any{profileTokenIDParam: profileTokenQueryValue, keyLabel: profileTokenLabel, keyConfirm: true}, want: errProfileTokenIDPositive},
		{name: "signed token id", args: map[string]any{profileTokenIDParam: "+12345", keyLabel: profileTokenLabel, keyConfirm: true}, want: errProfileTokenIDPositive},
		{name: "traversal token id", args: map[string]any{profileTokenIDParam: pathTraversalValue, keyLabel: profileTokenLabel, keyConfirm: true}, want: errProfileTokenIDPositive},
		{name: "missing fields", args: map[string]any{profileTokenIDParam: profileTokenIDFixture, keyConfirm: true}, want: "at least one profile token field is required"},
		{name: caseEmptyLabel, args: map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: "", keyConfirm: true}, want: databaseLabelRequiredMessage},
		{name: "empty expiry", args: map[string]any{profileTokenIDParam: profileTokenIDFixture, keyExpiry: "", keyConfirm: true}, want: "expiry must be a non-empty string"},
		{name: "numeric scopes", args: map[string]any{profileTokenIDParam: profileTokenIDFixture, profileTokenScopesField: 123, keyConfirm: true}, want: "scopes must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeProfileTokenUpdateTool(&config.Config{})
			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

			require.NoError(t, err, "validation failures are returned as tool errors")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid input should be rejected")
			assertErrorContains(t, result, testCase.want)
		})
	}
}

func TestLinodeProfileTokenUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		assert.Equal(t, profileTokenLabel, body[keyLabel])
		assert.Equal(t, profileTokenUpdatedScopes, body[profileTokenScopesField])
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel, profileTokenScopesField: profileTokenUpdatedScopes}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, profileTokenScopesField: profileTokenUpdatedScopes, keyConfirm: true}))

	require.NoError(t, err, "handler should not return a Go error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "success should not be an error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, profileTokenLabel, "response should include token label")
}

func TestLinodeProfileTokenUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(&config.Config{})
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: false, keyDryRun: true}))

	require.NoError(t, err, "dry-run should not return a Go error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "dry-run should not require confirm")
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "content should be TextContent")
	assert.Contains(t, textContent.Text, "PUT", "dry-run should show method")
	assert.Contains(t, textContent.Text, "/profile/tokens/12345", "dry-run should show path")
	assert.Contains(t, textContent.Text, profileTokenLabel, "dry-run should show body")
}

func TestLinodeProfileTokenUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
		assert.Equal(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: true}))

	require.NoError(t, err, "API failures are returned as tool errors")
	require.NotNil(t, result, "result should not be nil")
	assert.True(t, result.IsError, "upstream API error should be an error result")
	assertErrorContains(t, result, "Failed to update linode_profile_token_update")
	assertErrorContains(t, result, errForbidden)
}

func profileTokenUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
