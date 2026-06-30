package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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

func TestLinodeProfileTokensToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokensTool(cfg)

	if tool.Name != "linode_profile_token_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_token_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyPage]; !ok {
		t.Errorf("props missing key %v", keyPage)
	}

	if _, ok := props[keyPageSize]; !ok {
		t.Errorf("props missing key %v", keyPageSize)
	}
}

func TestLinodeProfileTokensToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{keyID: float64(67890), keyLabel: profileTokenLabel}},
			Page: 1, Pages: 1, Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, profileTokenLabel) {
		t.Errorf("textContent.Text does not contain %v", profileTokenLabel)
	}
}

func TestLinodeProfileTokensToolDropsSecret(t *testing.T) {
	t.Parallel()

	const tokenSecret = "abcdef0123456789-this-secret-must-not-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// The Linode list endpoint returns a token value; the metadata-only
		// proto element has no token field, so the DiscardUnknown decode must
		// drop it before the tool ever serializes a response.
		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ProfileToken]{
			Data: []linode.ProfileToken{{
				keyID: float64(67890), keyLabel: profileTokenLabel, keyToken: tokenSecret,
			}},
			Page: 1, Pages: 1, Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if strings.Contains(textContent.Text, tokenSecret) {
		t.Errorf("output leaked the token secret %q: %s", tokenSecret, textContent.Text)
	}

	if !strings.Contains(textContent.Text, profileTokenLabel) {
		t.Errorf("textContent.Text does not contain %v", profileTokenLabel)
	}
}

func TestLinodeProfileTokensToolInvalidPagination(t *testing.T) {
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
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Error("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeProfileTokensToolUpstreamError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeProfileTokenUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokenUpdateTool(cfg)

	if tool.Name != "linode_profile_token_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_token_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties

	if _, ok := props[profileTokenIDParam]; !ok {
		t.Errorf("props missing key %v", profileTokenIDParam)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	for _, field := range []string{keyExpiry, profileTokenScopesField} {
		if _, ok := props[field]; ok {
			t.Errorf("props has unexpected key %v (a token update only changes the label)", field)
		}
	}

	for _, key := range []string{profileTokenIDParam, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
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
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
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
		{name: caseMissingLabel, args: map[string]any{profileTokenIDParam: profileTokenIDFixture, keyConfirm: true}, want: errLabelRequired},
		{name: caseEmptyLabel, args: map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: "", keyConfirm: true}, want: databaseLabelRequiredMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeProfileTokenUpdateTool(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeProfileTokenUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should be JSON: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], profileTokenLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], profileTokenLabel)
		}

		if _, hasScopes := body[profileTokenScopesField]; hasScopes {
			t.Errorf("body should not contain %v (a token update only changes the label)", profileTokenScopesField)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ProfileToken{keyID: float64(12345), keyLabel: profileTokenLabel, profileTokenScopesField: profileTokenUpdatedScopes}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, profileTokenLabel) {
		t.Errorf("textContent.Text does not contain %v", profileTokenLabel)
	}
}

func TestLinodeProfileTokenUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: false, keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "PUT") {
		t.Errorf("textContent.Text does not contain %v", "PUT")
	}

	if !strings.Contains(textContent.Text, tcProfileTokens12345) {
		t.Errorf("textContent.Text does not contain %v", tcProfileTokens12345)
	}

	if !strings.Contains(textContent.Text, profileTokenLabel) {
		t.Errorf("textContent.Text does not contain %v", profileTokenLabel)
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "label is set to") {
		t.Errorf("textContent.Text does not contain %v", "label is set to")
	}
}

func TestLinodeProfileTokenUpdateAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcProfileTokens12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	_, _, handler := tools.NewLinodeProfileTokenUpdateTool(profileTokenUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{profileTokenIDParam: profileTokenIDFixture, keyLabel: profileTokenLabel, keyConfirm: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_profile_token_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_profile_token_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func profileTokenUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
