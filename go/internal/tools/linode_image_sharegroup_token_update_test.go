package tools_test

import (
	"context"
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
	imageShareGroupTokenUpdateToolName = "linode_image_sharegroup_token_update"
	updatedShareGroupTokenLabel        = "Backend Services - Engineering"
	shareGroupTokenUpdateCreated       = "2025-08-05T10:09:09"
	errTokenUUIDInvalid                = "token_uuid must be a UUID"
	errTokenUUIDUnsafe                 = "token_uuid must not contain path separators, query separators, fragments, or traversal segments"
	caseBlankLabelImageShareGroupToken = "blank label"
)

func TestLinodeImageShareGroupTokenUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(&config.Config{})

	if tool.Name != imageShareGroupTokenUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupTokenUpdateToolName)
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

	props := tool.InputSchema.Properties
	if _, ok := props[keyTokenUUID]; !ok {
		t.Errorf("props missing key %v", keyTokenUUID)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyTokenUUID, keyLabel, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeImageShareGroupTokenUpdateRequiresConfirm(t *testing.T) {
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

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupTokenUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyLabel: updatedShareGroupTokenLabel}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageShareGroupTokenUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing token uuid", args: map[string]any{keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "empty token uuid", args: map[string]any{keyTokenUUID: blankString, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "path separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithSlash, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "query separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithQuery, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "fragment separator token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithFragment, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "encoded separator token uuid", args: map[string]any{keyTokenUUID: "token%2Fuuid", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: "traversal token uuid", args: map[string]any{keyTokenUUID: tokenUUIDWithDotdot, keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "encoded traversal token uuid", args: map[string]any{keyTokenUUID: "token%2e%2euuid", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: "invalid uuid syntax", args: map[string]any{keyTokenUUID: "abc", keyLabel: updatedShareGroupTokenLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: caseMissingLabel, args: map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyTokenUUID: shareGroupUUIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: errLabelRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupTokenUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantContains)
			}

			if requestCount.Load() != int32(0) {
				t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageShareGroupTokenUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/images/sharegroups/tokens/"+shareGroupUUIDFixture {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens/"+shareGroupUUIDFixture)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyLabel], updatedShareGroupTokenLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], updatedShareGroupTokenLabel)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupToken{
			TokenUUID:              shareGroupUUIDFixture,
			Status:                 statusActive,
			Label:                  updatedShareGroupTokenLabel,
			Created:                shareGroupTokenUpdateCreated,
			ValidForShareGroupUUID: shareGroupUUIDFixture,
			ShareGroupUUID:         shareGroupUUIDFixture,
			ShareGroupLabel:        shareGroupLabelFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupUUIDFixture,
		keyLabel:     updatedShareGroupTokenLabel,
		keyConfirm:   true,
	}))
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

	if !strings.Contains(textContent.Text, shareGroupUUIDFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupUUIDFixture)
	}

	if !strings.Contains(textContent.Text, updatedShareGroupTokenLabel) {
		t.Errorf("textContent.Text does not contain %v", updatedShareGroupTokenLabel)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeImageShareGroupTokenUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"token not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyTokenUUID: shareGroupUUIDFixture,
		keyLabel:     updatedShareGroupTokenLabel,
		keyConfirm:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update image share group token") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update image share group token")
	}
}

func newImageShareGroupTokenUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(imageShareGroupTokenUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupTokenUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
