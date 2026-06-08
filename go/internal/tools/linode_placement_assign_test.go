package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyPlacementGroupLinodes   = "linodes"
	placementGroupIDFixture    = "528"
	placementGroupLinodeSingle = float64(123)
	errConfirmTrue             = "confirm=true"
	errLinodesRequired         = "linodes is required"
	keyPlacementGroupCompliant = "is_compliant"
	keyPlacementGroupMembers   = "members"
)

func TestLinodePlacementGroupAssignToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

	if tool.Name != "linode_placement_group_assign" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_placement_group_assign")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyPlacementGroupID, keyPlacementGroupLinodes, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodePlacementGroupAssignToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}}, wantContains: errConfirmTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: false}, wantContains: errConfirmTrue},
		{name: "string confirm", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: boolStringTrue}, wantContains: errConfirmTrue},
		{name: "numeric confirm", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: 1}, wantContains: errConfirmTrue},
		{name: caseMissingGroupID, args: map[string]any{keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDRequired},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "missing linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyConfirm: true}, wantContains: errLinodesRequired},
		{name: "dry run still validates linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyDryRun: true}, wantContains: errLinodesRequired},
		{name: "string linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: "[123]", keyConfirm: true}, wantContains: "linodes must be a JSON array"},
		{name: "invalid linode element", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{"123"}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "empty linodes", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{}, keyConfirm: true}, wantContains: "at least one"},
		{name: "zero linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(0)}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "fractional linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123.5)}, keyConfirm: true}, wantContains: errPositiveInteger},
		{name: "duplicate linode", args: map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123), float64(123)}, keyConfirm: true}, wantContains: "unique"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)
			req := createRequestWithArgs(t, tt.args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantContains) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantContains)
			}
		})
	}
}

func TestLinodePlacementGroupAssignToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/placement/groups/528/assign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/528/assign")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string][]int

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if !reflect.DeepEqual(body[keyPlacementGroupLinodes], []int{123, 456}) {
			t.Errorf("body[keyPlacementGroupLinodes] = %v, want %v", body[keyPlacementGroupLinodes], []int{123, 456})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
			keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
			keyPlacementGroupCompliant: true,
			keyPlacementGroupMembers:   []map[string]any{{keyLinodeID: 123, keyPlacementGroupCompliant: true}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{float64(123), float64(456)}, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "Assigned 2 Linode") {
		t.Errorf("textContent.Text does not contain %v", "Assigned 2 Linode")
	}

	if !strings.Contains(textContent.Text, placementGroupLabel) {
		t.Errorf("textContent.Text does not contain %v", placementGroupLabel)
	}
}

func TestLinodePlacementGroupAssignToolApiErrorIncludesGroupIdAndReason(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/placement/groups/528/assign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/528/assign")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyConfirm: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "placement group 528") {
		t.Errorf("error text %q does not contain %q", text.Text, "placement group 528")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodePlacementGroupAssignToolDryRunStateFetchErrorIsReported(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcPlacementGroups528 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodePlacementGroupAssignToolDryRunSkipsConfirmAndDoesNotPost(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcPlacementGroups528 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: 528, keyLabel: placementGroupLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodePlacementGroupAssignTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: placementGroupIDFixture, keyPlacementGroupLinodes: []any{placementGroupLinodeSingle}, keyDryRun: true})

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

	if !strings.Contains(textContent.Text, "/placement/groups/528/assign") {
		t.Errorf("textContent.Text does not contain %v", "/placement/groups/528/assign")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}

	if !strings.Contains(textContent.Text, placementGroupLabel) {
		t.Errorf("textContent.Text does not contain %v", placementGroupLabel)
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "assigned to placement group 528") {
		t.Errorf("textContent.Text does not contain %v", "assigned to placement group 528")
	}
}
