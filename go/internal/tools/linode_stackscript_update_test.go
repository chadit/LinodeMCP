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
	keyStackScriptIsPublic    = "is_public"
	errStackScriptID          = "stackscript_id must be a positive integer"
	stackScriptRevNoteUpdated = "revision update"
	stackScriptUpdateDesc     = "update description"
)

func TestLinodeStackScriptUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeStackScriptUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_stackscript_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_stackscript_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyStackScriptID, keyLabel, keyScript, keyImages, keyDescription, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeStackScriptUpdateToolValidation(t *testing.T) {
	t.Parallel()

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
		{name: "empty images", args: map[string]any{keyStackScriptID: 12345, keyImages: []any{" "}, keyConfirm: true}, wantContains: "images must contain at least one image ID"},
		{name: "query image", args: map[string]any{keyStackScriptID: 12345, keyImages: []any{configIDQueryValue}, keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "fragment image", args: map[string]any{keyStackScriptID: 12345, keyImages: []any{"linode/debian12#fragment"}, keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "extra separator image", args: map[string]any{keyStackScriptID: 12345, keyImages: []any{"private/15/extra"}, keyConfirm: true}, wantContains: errStackScriptImagesValid},
		{name: "traversal image", args: map[string]any{keyStackScriptID: 12345, keyImages: []any{privateImageTraversalFixture}, keyConfirm: true}, wantContains: errStackScriptImagesValid},
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeStackScriptUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	updated := linode.StackScript{ID: 12345, Label: testStackScriptLabel, Script: testStackScriptWithWhitespace, Images: []string{testDebian12Image}, RevNote: stackScriptRevNoteUpdated, IsPublic: true}

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/linode/stackscripts/12345" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/stackscripts/12345")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		for key, want := range map[string]any{
			keyLabel:               testStackScriptLabel,
			keyScript:              testStackScriptWithWhitespace,
			keyImages:              []any{testDebian12Image},
			keyDescription:         stackScriptUpdateDesc,
			keyStackScriptIsPublic: true,
			"rev_note":             stackScriptRevNoteUpdated,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
		keyImages:              []any{testDebian12Image},
		keyDescription:         stackScriptUpdateDesc,
		keyStackScriptIsPublic: true,
		"rev_note":             stackScriptRevNoteUpdated,
		keyConfirm:             true,
	})

	result, err := successHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, testStackScriptLabel) {
		t.Errorf("textContent.Text does not contain %v", testStackScriptLabel)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeStackScriptUpdateToolClientErrorPropagates(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"script invalid"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	errCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, errHandler := tools.NewLinodeStackScriptUpdateTool(errCfg)

	req := createRequestWithArgs(t, map[string]any{keyStackScriptID: 12345, keyLabel: testStackScriptLabel, keyConfirm: true})

	result, err := errHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update StackScript") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update StackScript")
	}
}
