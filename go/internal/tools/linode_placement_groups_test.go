package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	placementGroupIDKey             = "group_id"
	placementGroupUpdatedLabel      = "pg-renamed"
	placementGroupIDIntegerMessage  = "group_id must be an integer"
	placementGroupLabelBlankMessage = "label must be a non-empty string"
	placementGroupSlashID           = "12/34"
	caseNumericPlacementGroupLabel  = "numeric label"
)

func TestLinodePlacementGroupListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodePlacementGroupListTool(cfg)

	if tool.Name != "linode_placement_group_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_placement_group_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyPage, keyPageSize} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodePlacementGroupListToolSuccess(t *testing.T) {
	t.Parallel()

	groups := []linode.PlacementGroup{{ID: 123, Label: "pg-east", Region: regionUSEast, PlacementGroupType: "anti_affinity:local", PlacementGroupPolicy: "strict", IsCompliant: true}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcPlacementGroups {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    groups,
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "pg-east") {
		t.Errorf("textContent.Text does not contain %v", "pg-east")
	}

	if !strings.Contains(textContent.Text, "us-east") {
		t.Errorf("textContent.Text does not contain %v", "us-east")
	}
}

func TestLinodePlacementGroupListToolInvalidPageSize(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodePlacementGroupListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}
}

func TestLinodePlacementGroupUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

	if tool.Name != "linode_placement_group_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_placement_group_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{placementGroupIDKey, keyLabel, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{placementGroupIDKey, keyLabel, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodePlacementGroupUpdateToolRequiresConfirm(t *testing.T) {
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

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatalf("confirm failure must happen before client call")
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

			args := map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel}
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
		})
	}
}

func TestLinodePlacementGroupUpdateToolInvalidRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing group_id", args: map[string]any{keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDRequired},
		{name: "zero group_id", args: map[string]any{placementGroupIDKey: 0, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: "group_id must be an integer greater than or equal to 1"},
		{name: "string group_id", args: map[string]any{placementGroupIDKey: "123", keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
		{name: "fractional group_id", args: map[string]any{placementGroupIDKey: 123.5, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
		{name: "slash group_id", args: map[string]any{placementGroupIDKey: placementGroupSlashID, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
		{name: "query group_id", args: map[string]any{placementGroupIDKey: "12?x=1", keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
		{name: "dotdot group_id", args: map[string]any{placementGroupIDKey: pathTraversalValue, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}, wantMessage: placementGroupIDIntegerMessage},
		{name: caseEmptyLabel, args: map[string]any{placementGroupIDKey: 123, keyLabel: "", keyConfirm: true}, wantMessage: placementGroupLabelBlankMessage},
		{name: caseNumericPlacementGroupLabel, args: map[string]any{placementGroupIDKey: 123, keyLabel: 123, keyConfirm: true}, wantMessage: placementGroupLabelBlankMessage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Fatalf("request validation must happen before client call")
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}
		})
	}
}

func TestLinodePlacementGroupUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyDryRun: true}))
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

	if !strings.Contains(textContent.Text, "linode_placement_group_update") {
		t.Errorf("textContent.Text does not contain %v", "linode_placement_group_update")
	}

	if !strings.Contains(textContent.Text, "PUT") {
		t.Errorf("textContent.Text does not contain %v", "PUT")
	}

	if !strings.Contains(textContent.Text, "/placement/groups/123") {
		t.Errorf("textContent.Text does not contain %v", "/placement/groups/123")
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "label is set to") {
		t.Errorf("textContent.Text does not contain %v", "label is set to")
	}
}

func TestLinodePlacementGroupUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/placement/groups/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/123")
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

		if !reflect.DeepEqual(body[keyLabel], placementGroupUpdatedLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], placementGroupUpdatedLabel)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 123, Label: placementGroupUpdatedLabel, Region: regionUSEast}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}))
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

	if !strings.Contains(textContent.Text, placementGroupUpdatedLabel) {
		t.Errorf("textContent.Text does not contain %v", placementGroupUpdatedLabel)
	}
}

func TestLinodePlacementGroupUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/placement/groups/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/123")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUpdateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{placementGroupIDKey: 123, keyLabel: placementGroupUpdatedLabel, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_placement_group_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_placement_group_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}
