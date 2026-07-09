package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	profileTokenLabelParam      = "label"
	profileTokenScopesParam     = "scopes"
	profileTokenLabelFixture    = "ci-token"
	profileTokenScopesFixture   = "linodes:read_only"
	profileTokenSecretFixture   = "secret-token-value"
	profileTokenExpiryFixture   = "2024-06-01T00:01:01"
	profileTokenCreatedFixture  = "2024-05-01T00:01:01"
	profileTokenConfirmRequired = "confirm=true"
	caseInvalidExpiry           = "invalid_expiry_type"
	caseInvalidLabel            = "invalid_label_type"
	caseInvalidScopes           = "invalid_scopes_type"
)

func TestLinodeProfileTokenCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

	if tool.Name != "linode_profile_token_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_token_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keyExpiry) {
		t.Errorf("tool.RawInputSchema missing key %v", keyExpiry)
	}

	if !strings.Contains(raw, profileTokenLabelParam) {
		t.Errorf("tool.RawInputSchema missing key %v", profileTokenLabelParam)
	}

	if !strings.Contains(raw, profileTokenScopesParam) {
		t.Errorf("tool.RawInputSchema missing key %v", profileTokenScopesParam)
	}

	if !strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}

	if !strings.Contains(raw, keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}

	if !strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeProfileTokenCreateToolSuccess(t *testing.T) {
	t.Parallel()

	createdToken := linode.ProfileToken{
		keyCreated:              profileTokenCreatedFixture,
		keyExpiry:               profileTokenExpiryFixture,
		keyID:                   float64(321),
		keyLabel:                profileTokenLabelFixture,
		profileTokenScopesParam: profileTokenScopesFixture,
		keyToken:                profileTokenSecretFixture,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreateProfileTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Expiry != profileTokenExpiryFixture {
			t.Errorf("got.Expiry = %v, want %v", got.Expiry, profileTokenExpiryFixture)
		}

		if got.Label != profileTokenLabelFixture {
			t.Errorf("got.Label = %v, want %v", got.Label, profileTokenLabelFixture)
		}

		if got.Scopes != profileTokenScopesFixture {
			t.Errorf("got.Scopes = %v, want %v", got.Scopes, profileTokenScopesFixture)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(createdToken); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyExpiry:               profileTokenExpiryFixture,
		profileTokenLabelParam:  profileTokenLabelFixture,
		profileTokenScopesParam: profileTokenScopesFixture,
		keyConfirm:              true,
	})

	result, err := handler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, profileTokenLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", profileTokenLabelFixture)
	}

	if !strings.Contains(textContent.Text, profileTokenScopesFixture) {
		t.Errorf("textContent.Text does not contain %v", profileTokenScopesFixture)
	}

	if !strings.Contains(textContent.Text, profileTokenSecretFixture) {
		t.Errorf("textContent.Text does not contain %v", profileTokenSecretFixture)
	}

	// The one-time token secret is returned by design and the response carries
	// the save-the-secret warning byte-identically to the Python implementation.
	if !strings.Contains(textContent.Text, "The token below is shown ONLY ONCE") {
		t.Errorf("response %q does not carry the save-the-secret warning", textContent.Text)
	}
}

func TestLinodeProfileTokenCreateToolApiErrorReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		profileTokenLabelParam: profileTokenLabelFixture,
		keyConfirm:             true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_profile_token_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_profile_token_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileTokenCreateToolConfirmGuardRejectsBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

			args := map[string]any{profileTokenLabelParam: profileTokenLabelFixture}
			if testCase.set {
				args[keyConfirm] = testCase.confirm
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

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Error("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, profileTokenConfirmRequired) {
				t.Errorf("textContent.Text does not contain %v", profileTokenConfirmRequired)
			}
		})
	}
}

func TestLinodeProfileTokenCreateToolInvalidOptionalFieldRejectsBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		field   string
		message string
	}{
		{name: caseInvalidExpiry, field: keyExpiry, message: "expiry must be a string"},
		{name: caseInvalidLabel, field: profileTokenLabelParam, message: errLabelString},
		{name: caseInvalidScopes, field: profileTokenScopesParam, message: "scopes must be a string"},
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
			_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

			args := map[string]any{keyConfirm: true, testCase.field: 123}

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

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.message) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.message)
			}
		})
	}
}

func profileTokenTestConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

func TestLinodeProfileTokenCreateToolDryRun(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := profileTokenTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyExpiry:               profileTokenExpiryFixture,
		profileTokenLabelParam:  profileTokenLabelFixture,
		profileTokenScopesParam: profileTokenScopesFixture,
		keyDryRun:               true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Error("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], tcProfileTokens) {
		t.Errorf("got %v, want %v", would["path"], tcProfileTokens)
	}

	wouldBody, bodyOK := would["body"].(map[string]any)
	if !bodyOK {
		t.Error("bodyOK = false, want true")
	}

	if !reflect.DeepEqual(wouldBody[profileTokenLabelParam], profileTokenLabelFixture) {
		t.Errorf("wouldBody[profileTokenLabelParam] = %v, want %v", wouldBody[profileTokenLabelParam], profileTokenLabelFixture)
	}

	if requestCount.Load() != int32(0) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Error("gotString = false, want true")
	}

	if !strings.Contains(effect, profileTokenLabelFixture) {
		t.Errorf("effect does not contain %v", profileTokenLabelFixture)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Errorf("len(warnings) = %d, want %d", len(warnings), 1)
	}

	warning, gotWarn := warnings[0].(string)
	if !gotWarn {
		t.Error("gotWarn = false, want true")
	}

	if !strings.Contains(warning, "once") {
		t.Errorf("warning does not contain %v", "once")
	}
}
