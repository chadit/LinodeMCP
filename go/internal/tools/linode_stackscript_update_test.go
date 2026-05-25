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
	keyStackScriptIsPublic    = "is_public"
	errStackScriptID          = "stackscript_id must be a positive integer"
	stackScriptRevNoteUpdated = "revision update"
	stackScriptUpdateDesc     = "update description"
)

func TestLinodeStackScriptUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeStackScriptUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_stackscript_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "StackScript update should be write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyStackScriptID, "schema should include stackscript_id property")
		assert.Contains(t, props, keyLabel, "schema should include label property")
		assert.Contains(t, props, keyScript, "schema should include script property")
		assert.Contains(t, props, keyImages, "schema should include images property")
		assert.Contains(t, props, keyDescription, "schema should include description property")
		assert.Contains(t, props, keyConfirm, "schema should include confirm property")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel, keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissing, args: map[string]any{keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseZero, args: map[string]any{keyStackScriptID: 0, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseNegativeLinodeID, args: map[string]any{keyStackScriptID: -1, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseNumeric, args: map[string]any{keyStackScriptID: 1.5, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseSlash, args: map[string]any{keyStackScriptID: pathSeparatorValue, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseQuery, args: map[string]any{keyStackScriptID: pathQueryValue, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseDotdot, args: map[string]any{keyStackScriptID: pathTraversalValue, keyLabel: testStackScriptLabel, keyConfirm: true}, wantContains: errStackScriptID},
		{name: caseNoUpdateFields, args: map[string]any{keyStackScriptID: 12345, keyConfirm: true}, wantContains: "at least one editable field is required"},
		{name: "empty label", args: map[string]any{keyStackScriptID: 12345, keyLabel: " ", keyConfirm: true}, wantContains: databaseLabelRequiredMessage},
		{name: "empty script", args: map[string]any{keyStackScriptID: 12345, keyScript: " ", keyConfirm: true}, wantContains: "script must be a non-empty string"},
		{name: "empty images", args: map[string]any{keyStackScriptID: 12345, keyImages: " , ", keyConfirm: true}, wantContains: "images must contain at least one image ID"},
		{name: "query image", args: map[string]any{keyStackScriptID: 12345, keyImages: configIDQueryValue, keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "fragment image", args: map[string]any{keyStackScriptID: 12345, keyImages: "linode/debian12#fragment", keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "extra separator image", args: map[string]any{keyStackScriptID: 12345, keyImages: "private/15/extra", keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "traversal image", args: map[string]any{keyStackScriptID: 12345, keyImages: privateImageTraversalFixture, keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "invalid public flag", args: map[string]any{keyStackScriptID: 12345, keyStackScriptIsPublic: boolStringTrue, keyConfirm: true}, wantContains: keyStackScriptIsPublic + " must be a boolean"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				requestCount.Add(1)
			}))
			t.Cleanup(srv.Close)

			validationCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, validationHandler := tools.NewLinodeStackScriptUpdateTool(validationCfg)

			req := createRequestWithArgs(t, tt.args)
			result, err := validationHandler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
			assert.Equal(t, int32(0), requestCount.Load(), "validation should reject before client call")
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		updated := linode.StackScript{ID: 12345, Label: testStackScriptLabel, Script: testStackScriptWithWhitespace, Images: []string{testDebian12Image}, RevNote: stackScriptRevNoteUpdated, IsPublic: true}

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, "/linode/stackscripts/12345", r.URL.Path, "request path should match StackScript endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, testStackScriptLabel, body[keyLabel], "label should be sent")
			assert.Equal(t, testStackScriptWithWhitespace, body[keyScript], "script should preserve exact content")
			assert.Equal(t, []any{testDebian12Image}, body[keyImages], "images should be sent")
			assert.Equal(t, stackScriptUpdateDesc, body[keyDescription], "description should be sent")
			assert.Equal(t, true, body[keyStackScriptIsPublic], "is_public should be sent")
			assert.Equal(t, stackScriptRevNoteUpdated, body["rev_note"], "rev_note should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(updated), "encoding response should succeed")
		}))
		t.Cleanup(srv.Close)

		successCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, successHandler := tools.NewLinodeStackScriptUpdateTool(successCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyStackScriptID:       12345,
			keyLabel:               testStackScriptLabel,
			keyScript:              testStackScriptWithWhitespace,
			keyImages:              testDebian12Image,
			keyDescription:         stackScriptUpdateDesc,
			keyStackScriptIsPublic: true,
			"rev_note":             stackScriptRevNoteUpdated,
			keyConfirm:             true,
		})
		result, err := successHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be an error")
		assert.Equal(t, int32(1), requestCount.Load(), "handler should call the client once")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, testStackScriptLabel, "response should contain the StackScript label")
		assert.Contains(t, textContent.Text, "updated successfully", "response should confirm update")
	})

	t.Run("client error propagates", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"reason":"script invalid"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, errHandler := tools.NewLinodeStackScriptUpdateTool(errCfg)

		req := createRequestWithArgs(t, map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel, keyConfirm: true})
		result, err := errHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update StackScript")
	})
}
