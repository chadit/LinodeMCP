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
	imageShareGroupMemberUpdateToolName = "linode_image_sharegroup_member_token_update"
	updatedShareGroupMemberLabel        = "Engineering - Backend"
)

func TestLinodeImageShareGroupMemberUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(&config.Config{})

	if tool.Name != imageShareGroupMemberUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupMemberUpdateToolName)
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
	if _, ok := props[keyShareGroupID]; !ok {
		t.Errorf("props missing key %v", keyShareGroupID)
	}

	if _, ok := props[keyTokenUUID]; !ok {
		t.Errorf("props missing key %v", keyTokenUUID)
	}

	if _, ok := props[keyLabel]; !ok {
		t.Errorf("props missing key %v", keyLabel)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyShareGroupID, keyTokenUUID, keyLabel, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}
}

func TestLinodeImageShareGroupMemberUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageShareGroupMemberUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel}
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

func TestLinodeImageShareGroupMemberUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingShareGroupID, args: map[string]any{keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: caseZeroShareGroupID, args: map[string]any{keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "path separator sharegroup id", args: map[string]any{keyShareGroupID: paymentMethodIDSlash, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "query separator sharegroup id", args: map[string]any{keyShareGroupID: shareGroupIDQueryValue, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: caseTraversalShareGroupID, args: map[string]any{keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "missing token uuid", args: map[string]any{keyShareGroupID: 123, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "empty token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: blankString, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDNonEmpty},
		{name: "path separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithSlash, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "query separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithQuery, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "fragment separator token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithFragment, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "traversal token uuid", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithDotdot, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDUnsafe},
		{name: "invalid uuid syntax", args: map[string]any{keyShareGroupID: 123, keyTokenUUID: invalidTokenUUID, keyLabel: updatedShareGroupMemberLabel, keyConfirm: true}, wantContains: errTokenUUIDInvalid},
		{name: caseMissingLabel, args: map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID, keyLabel: blankString, keyConfirm: true}, wantContains: errLabelRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupMemberUpdateHandler(t, &requestCount)
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

func TestLinodeImageShareGroupMemberUpdateSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/images/sharegroups/123/members/"+shareGroupTokenGetUUID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/members/"+shareGroupTokenGetUUID)
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

		if !reflect.DeepEqual(body[keyLabel], updatedShareGroupMemberLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], updatedShareGroupMemberLabel)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{
			TokenUUID: shareGroupTokenGetUUID,
			Status:    statusActive,
			Label:     updatedShareGroupMemberLabel,
			Created:   imageShareGroupTokenCreated,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 123,
		keyTokenUUID:    shareGroupTokenGetUUID,
		keyLabel:        updatedShareGroupMemberLabel,
		keyConfirm:      true,
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

	if !strings.Contains(textContent.Text, shareGroupTokenGetUUID) {
		t.Errorf("textContent.Text does not contain %v", shareGroupTokenGetUUID)
	}

	if !strings.Contains(textContent.Text, updatedShareGroupMemberLabel) {
		t.Errorf("textContent.Text does not contain %v", updatedShareGroupMemberLabel)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeImageShareGroupMemberUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"member not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 123,
		keyTokenUUID:    shareGroupTokenGetUUID,
		keyLabel:        updatedShareGroupMemberLabel,
		keyConfirm:      true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update image share group member token") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update image share group member token")
	}
}

func newImageShareGroupMemberUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(imageShareGroupMemberUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupMemberUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
