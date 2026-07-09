package tools_test

import (
	"context"
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
	imageShareGroupUpdateToolName    = "linode_image_sharegroup_update"
	imageShareGroupIDParam           = "sharegroup_id"
	imageShareGroupIDFixture         = 54321
	updatedImageShareGroupLabel      = "Engineering Base Images"
	updatedImageShareGroupDesc       = "Base images used by engineering teams"
	errImageShareGroupIDPositive     = "sharegroup_id must be a positive integer"
	errImageShareGroupUpdateRequired = "at least one of label or description is required"
)

func TestLinodeImageShareGroupUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupUpdateTool(&config.Config{})

	if tool.Name != imageShareGroupUpdateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupUpdateToolName)
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
	for _, key := range []string{imageShareGroupIDParam, keyLabel, keyDescription, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeImageShareGroupUpdateRequiresConfirm(t *testing.T) {
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

			handler, cleanup := newImageShareGroupUpdateHandler(t, &requestCount)
			t.Cleanup(cleanup)

			args := map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: updatedImageShareGroupLabel}
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

func TestLinodeImageShareGroupUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing share group id", args: map[string]any{keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errShareGroupIDRequired},
		{name: "zero share group id", args: map[string]any{imageShareGroupIDParam: 0, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "negative share group id", args: map[string]any{imageShareGroupIDParam: -1, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "fractional share group id", args: map[string]any{imageShareGroupIDParam: 1.25, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "string share group id", args: map[string]any{imageShareGroupIDParam: "not-a-number", keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "path separator share group id", args: map[string]any{imageShareGroupIDParam: paymentMethodIDSlash, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "query separator share group id", args: map[string]any{imageShareGroupIDParam: paymentMethodIDQuery, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "traversal share group id", args: map[string]any{imageShareGroupIDParam: pathTraversalValue, keyLabel: updatedImageShareGroupLabel, keyConfirm: true}, wantContains: errImageShareGroupIDPositive},
		{name: "missing update fields", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyConfirm: true}, wantContains: errImageShareGroupUpdateRequired},
		{name: caseBlankLabelImageShareGroupToken, args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyLabel: blankString, keyConfirm: true}, wantContains: "label must be a non-empty string"},
		{name: "blank description", args: map[string]any{imageShareGroupIDParam: imageShareGroupIDFixture, keyDescription: blankString, keyConfirm: true}, wantContains: "description must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var requestCount atomic.Int32

			handler, cleanup := newImageShareGroupUpdateHandler(t, &requestCount)
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

func TestLinodeImageShareGroupUpdateSuccess(t *testing.T) {
	t.Parallel()

	description := updatedImageShareGroupDesc

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/images/sharegroups/54321" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/54321")
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

		if !reflect.DeepEqual(body[keyLabel], updatedImageShareGroupLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], updatedImageShareGroupLabel)
		}

		if !reflect.DeepEqual(body[keyDescription], description) {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], description)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           imageShareGroupIDFixture,
			UUID:         shareGroupUUIDFixture,
			Label:        updatedImageShareGroupLabel,
			Description:  &description,
			IsSuspended:  false,
			Created:      imageShareGroupCreated,
			Updated:      &description,
			ImagesCount:  2,
			MembersCount: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam: imageShareGroupIDFixture,
		keyLabel:               updatedImageShareGroupLabel,
		keyDescription:         description,
		keyConfirm:             true,
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

	if !strings.Contains(textContent.Text, updatedImageShareGroupLabel) {
		t.Errorf("textContent.Text does not contain %v", updatedImageShareGroupLabel)
	}

	if !strings.Contains(textContent.Text, "updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "updated successfully")
	}
}

func TestLinodeImageShareGroupUpdateClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		_, err := w.Write([]byte(`{"errors":[{"reason":"share group not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		imageShareGroupIDParam: imageShareGroupIDFixture,
		keyLabel:               updatedImageShareGroupLabel,
		keyConfirm:             true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update image share group") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update image share group")
	}
}

func newImageShareGroupUpdateHandler(t *testing.T, requestCount *atomic.Int32) (func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(imageShareGroupUpdateConfig(srv.URL))

	return handler, srv.Close
}

func imageShareGroupUpdateConfig(apiURL string) *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest}},
	}}
}
