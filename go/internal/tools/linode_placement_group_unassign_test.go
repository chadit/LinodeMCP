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

const placementGroupUnassignToolName = "linode_placement_group_unassign"

func TestLinodePlacementGroupUnassignToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

	if tool.Name != placementGroupUnassignToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, placementGroupUnassignToolName)
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
	for _, key := range []string{keyPlacementGroupID, keyPlacementGroupLinodes, keyDryRun, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodePlacementGroupUnassignToolConfirmRequiredBeforeClientCall(t *testing.T) {
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

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

			args := placementGroupUnassignArgs()
			delete(args, keyConfirm)

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

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodePlacementGroupUnassignToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		update      func(map[string]any)
		wantMessage string
	}{
		{name: "missing group_id", update: func(args map[string]any) { delete(args, keyPlacementGroupID) }, wantMessage: "group_id is required"},
		{name: "slash group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = pathSeparatorValue }, wantMessage: placementGroupIDError},
		{name: "query group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = "12?x=1" }, wantMessage: placementGroupIDError},
		{name: "traversal group_id", update: func(args map[string]any) { args[keyPlacementGroupID] = pathTraversalValue }, wantMessage: placementGroupIDError},
		{name: "missing linodes", update: func(args map[string]any) { delete(args, "linodes") }, wantMessage: tools.ErrPlacementGroupLinodesRequired.Error()},
		{name: "empty linodes", update: func(args map[string]any) { args["linodes"] = []any{} }, wantMessage: tools.ErrPlacementGroupLinodesEmpty.Error()},
		{name: "string linodes", update: func(args map[string]any) { args["linodes"] = []any{"123"} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
		{name: "non-array linodes", update: func(args map[string]any) { args["linodes"] = "123" }, wantMessage: tools.ErrPlacementGroupLinodesJSON.Error()},
		{name: "fractional linode", update: func(args map[string]any) { args["linodes"] = []any{123.5} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
		{name: "duplicate linode", update: func(args map[string]any) { args["linodes"] = []any{float64(123), float64(123)} }, wantMessage: tools.ErrPlacementGroupLinodesDuplicate.Error()},
		{name: "zero linode", update: func(args map[string]any) { args["linodes"] = []any{float64(0)} }, wantMessage: tools.ErrPlacementGroupLinodesPositive.Error()},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

			args := placementGroupUnassignArgs()
			testCase.update(args)

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

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodePlacementGroupUnassignToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/placement/groups/789/unassign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/789/unassign")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupUnassignArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to unassign placement group") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to unassign placement group")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodePlacementGroupUnassignToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/placement/groups/789/unassign" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/789/unassign")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.PlacementGroupUnassignRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got.Linodes, []int{123, 456}) {
			t.Errorf("got.Linodes = %v, want %v", got.Linodes, []int{123, 456})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 789, Label: "pg-test", Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupUnassignArgs()))
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

	if !strings.Contains(textContent.Text, "pg-test") {
		t.Errorf("textContent.Text does not contain %v", "pg-test")
	}

	if !strings.Contains(textContent.Text, "unassigned") {
		t.Errorf("textContent.Text does not contain %v", "unassigned")
	}
}

func placementGroupUnassignArgs() map[string]any {
	return map[string]any{
		keyPlacementGroupID: float64(789),
		"linodes":           []any{float64(123), float64(456)},
		keyConfirm:          true,
	}
}

func TestLinodePlacementGroupUnassignToolDryRun(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/placement/groups/789" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/789")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 789, Label: "pg-test", Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupUnassignTool(cfg)

	args := placementGroupUnassignArgs()
	delete(args, keyConfirm)
	args[keyDryRun] = true

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
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

	body := decodeBody(t, textContent.Text)
	if body["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", body["dry_run"])
	}

	assertDryRunRequest(t, body, "POST", "/placement/groups/789/unassign")

	would, _ := body["would_execute"].(map[string]any)

	reqBody, hasBody := would["body"].(map[string]any)
	if !hasBody {
		t.Fatalf("would_execute.body is not an object: %v", would["body"])
	}

	linodes, isList := reqBody["linodes"].([]any)
	if !isList || len(linodes) != 2 {
		t.Fatalf("body.linodes = %v, want two entries", reqBody["linodes"])
	}

	if linodes[0] != float64(123) || linodes[1] != float64(456) {
		t.Errorf("body.linodes = %v, want [123 456]", linodes)
	}

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "removed from placement group 789") {
		t.Errorf("textContent.Text does not contain %v", "removed from placement group 789")
	}

	if calls != int32(1) {
		t.Errorf("calls = %v, want %v", calls, int32(1))
	}
}
