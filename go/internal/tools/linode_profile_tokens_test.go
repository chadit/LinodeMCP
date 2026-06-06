package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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

		checkEqual(t, "linode_profile_tokens", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		checkEqual(t, profiles.CapRead, capability, "profile tokens should be read-only")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, keyPage, "schema should expose optional page")
		expectContainsWithMode(t, false, props, keyPageSize, "schema should expose optional page_size")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
			checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
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

		expectNoError(t, err, "handler should not return a Go error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "success should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, profileTokenLabel, "response should include token label")
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

				expectNoError(t, err, "validation failures are returned as error results")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "invalid pagination should be rejected before client call")
				textContent, ok := result.Content[0].(mcp.TextContent)
				expectTrue(t, ok, "content should be TextContent")
				expectContainsWithMode(t, false, textContent.Text, testCase.wantMessage, "response should describe pagination validation")
			})
		}
	})

	t.Run("upstream error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
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

		expectNoError(t, err, "upstream failures are returned as error results")
		expectNotNil(t, result, "result should not be nil")
		checkTrueWithMode(t, false, result.IsError, "upstream API error should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, "Failed to retrieve linode_profile_tokens")
	})
}

func TestLinodeProfileTokenUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokenUpdateTool(cfg)

	checkEqual(t, "linode_profile_token_update", tool.Name, "tool name should match")
	checkEqual(t, profiles.CapAdmin, capability, "profile token update should be admin-capable")
	expectNotNil(t, handler, "handler should not be nil")

	props := tool.InputSchema.Properties

	expectContainsWithMode(t, false, props, profileTokenIDParam, "schema should include token_id")
	expectContainsWithMode(t, false, props, keyConfirm, "schema should include confirm")

	for _, field := range []string{keyExpiry, keyLabel, profileTokenScopesField} {
		expectContainsWithMode(t, false, props, field, "schema should include documented body field")
	}

	expectContainsWithMode(t, false, tool.InputSchema.Required, profileTokenIDParam, "token_id must be required")
	expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfirm, "confirm must be required")
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

			expectNoError(t, err, "confirm failures are returned as tool errors")
			expectNotNil(t, result, "result should not be nil")
			checkTrueWithMode(t, false, result.IsError, "confirm failure should be an error result")
			assertErrorContains(t, result, errConfirmEqualsTrue)
			checkEqual(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
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

			expectNoError(t, err, "validation failures are returned as tool errors")
			expectNotNil(t, result, "result should not be nil")
			checkTrueWithMode(t, false, result.IsError, "invalid input should be rejected")
			assertErrorContains(t, result, testCase.want)
		})
	}
}

func TestLinodeProfileTokenUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")

		var body map[string]any
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON") {
			return
		}

		checkEqual(t, profileTokenLabel, body[keyLabel])
		checkEqual(t, profileTokenUpdatedScopes, body[profileTokenScopesField])
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel, profileTokenScopesField: profileTokenUpdatedScopes}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, profileTokenScopesField: profileTokenUpdatedScopes, keyConfirm: true}))

	expectNoError(t, err, "handler should not return a Go error")
	expectNotNil(t, result, "result should not be nil")
	checkFalseWithMode(t, false, result.IsError, "success should not be an error result")
	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok, "content should be TextContent")
	expectContainsWithMode(t, false, textContent.Text, profileTokenLabel, "response should include token label")
}

func TestLinodeProfileTokenUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(&config.Config{})
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: false, keyDryRun: true}))

	expectNoError(t, err, "dry-run should not return a Go error")
	expectNotNil(t, result, "result should not be nil")
	checkFalseWithMode(t, false, result.IsError, "dry-run should not require confirm")
	textContent, ok := result.Content[0].(mcp.TextContent)
	expectTrue(t, ok, "content should be TextContent")
	expectContainsWithMode(t, false, textContent.Text, "PUT", "dry-run should show method")
	expectContainsWithMode(t, false, textContent.Text, "/profile/tokens/12345", "dry-run should show path")
	expectContainsWithMode(t, false, textContent.Text, profileTokenLabel, "dry-run should show body")
	expectContainsWithMode(t, false, textContent.Text, "side_effects", "dry-run should surface side effects")
	expectContainsWithMode(t, false, textContent.Text, "label is set to", "side effect should describe the label change")
}

func TestLinodeProfileTokenUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/profile/tokens/12345", r.URL.Path, "request path should include token ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))
	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: true}))

	expectNoError(t, err, "API failures are returned as tool errors")
	expectNotNil(t, result, "result should not be nil")
	checkTrueWithMode(t, false, result.IsError, "upstream API error should be an error result")
	assertErrorContains(t, result, "Failed to update linode_profile_token_update")
	assertErrorContains(t, result, errForbidden)
}

func profileTokenUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
