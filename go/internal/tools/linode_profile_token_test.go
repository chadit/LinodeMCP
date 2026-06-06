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

func TestLinodeProfileTokenCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

		checkEqual(t, "linode_profile_token_create", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapAdmin, capability, "tool should be admin capability")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContainsWithMode(t, false, props, keyExpiry, "schema should include expiry")
		expectContainsWithMode(t, false, props, profileTokenLabelParam, "schema should include label")
		expectContainsWithMode(t, false, props, profileTokenScopesParam, "schema should include scopes")
		expectContainsWithMode(t, false, props, keyConfirm, "mutating token tool must require confirm")
		expectContainsWithMode(t, false, props, keyDryRun, "admin tool should expose dry_run")
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
	})

	t.Run("success", func(t *testing.T) {
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
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got linode.CreateProfileTokenRequest
			checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should be valid JSON")
			checkEqual(t, profileTokenExpiryFixture, got.Expiry)
			checkEqual(t, profileTokenLabelFixture, got.Label)
			checkEqual(t, profileTokenScopesFixture, got.Scopes)

			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(createdToken))
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

		expectNoError(t, err, "handler should not return an error")
		expectNotNil(t, result, "result should not be nil")
		checkFalseWithMode(t, false, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContainsWithMode(t, false, textContent.Text, profileTokenLabelFixture, "response should contain token label")
		expectContainsWithMode(t, false, textContent.Text, profileTokenScopesFixture, "response should contain token scopes")
		expectContainsWithMode(t, false, textContent.Text, profileTokenSecretFixture, "response should contain token value")
	})

	t.Run("api error returns tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			checkEqual(t, "/profile/tokens", r.URL.Path, "request path should be /profile/tokens")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := profileTokenTestConfig(srv.URL)
		_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			profileTokenLabelParam: profileTokenLabelFixture,
			keyConfirm:             true,
		}))

		expectNoError(t, err, "handler should return API failures as tool errors")
		expectNotNil(t, result, "result should not be nil")
		checkTrueWithMode(t, false, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_profile_token_create")
		assertErrorContains(t, result, errForbidden)
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
				_, _, handler := tools.NewLinodeProfileTokenCreateTool(cfg)

				args := map[string]any{profileTokenLabelParam: profileTokenLabelFixture}
				if testCase.set {
					args[keyConfirm] = testCase.confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				expectNoError(t, err, "handler should return validation as a tool result")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "invalid confirm should be an error result")
				checkEqual(t, int32(0), requestCount.Load(), "client must not be called when confirm is invalid")

				textContent, ok := result.Content[0].(mcp.TextContent)
				expectTrue(t, ok, "content should be TextContent")
				expectContainsWithMode(t, false, textContent.Text, profileTokenConfirmRequired, "response should describe confirm requirement")
			})
		}
	})

	t.Run("invalid optional field rejects before client call", func(t *testing.T) {
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

				expectNoError(t, err, "handler should return validation as a tool result")
				expectNotNil(t, result, "result should not be nil")
				checkTrueWithMode(t, false, result.IsError, "invalid optional field should be an error result")
				checkEqual(t, int32(0), requestCount.Load(), "client must not be called when an optional field is invalid")
				assertErrorContains(t, result, testCase.message)
			})
		}
	})
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

	expectNoError(t, err, "handler should not return an error")
	expectNotNil(t, result, "result should not be nil")
	checkFalseWithMode(t, false, result.IsError, "dry run should not be an error result")

	var body map[string]any
	expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	checkEqual(t, true, body[keyDryRun], "response should be a dry-run preview")

	would, wouldOK := body["would_execute"].(map[string]any)
	expectTrue(t, wouldOK, "dry run response should include would_execute")
	checkEqual(t, "POST", would["method"])
	checkEqual(t, "/profile/tokens", would["path"])

	wouldBody, bodyOK := would["body"].(map[string]any)
	expectTrue(t, bodyOK, "dry run response should include request body")
	checkEqual(t, profileTokenLabelFixture, wouldBody[profileTokenLabelParam])
	checkEqual(t, int32(0), requestCount.Load(), "dry run should not call the POST endpoint")

	sideEffects, _ := body["side_effects"].([]any)
	expectLen(t, sideEffects, 1, "token create surfaces a side effect")

	effect, gotString := sideEffects[0].(string)
	expectTrue(t, gotString)
	expectContainsWithMode(t, false, effect, profileTokenLabelFixture, "side effect should name the token")

	warnings, _ := body["warnings"].([]any)
	expectLen(t, warnings, 1, "token create warns the secret is shown once")

	warning, gotWarn := warnings[0].(string)
	expectTrue(t, gotWarn)
	expectContainsWithMode(t, false, warning, "once", "warning should flag the one-time secret")
}
