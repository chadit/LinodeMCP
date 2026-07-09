package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	placementGroupCreateToolName = "linode_placement_group_create"
	placementGroupCreateLabel    = "pg-test"
	placementGroupCreateRegion   = "us-east"
	placementGroupType           = "anti_affinity:local"
	errPlacementGroupRegionBlank = "region must be a non-empty string"
	caseNumericLabel             = "numeric label"
	placementGroupCreatePolicy   = "strict"
)

func TestLinodePlacementGroupCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

	if tool.Name != placementGroupCreateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, placementGroupCreateToolName)
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
	for _, key := range []string{
		managedServiceLabelParam,
		keySupportTicketRegion,
		keyPlacementGroupTypeJSON,
		keyPlacementGroupPolicyJSON,
		keyDryRun,
		keyConfirm,
	} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodePlacementGroupCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
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
			_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

			args := placementGroupCreateArgs()
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

func TestLinodePlacementGroupCreateToolInvalidRequestRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		update      func(map[string]any)
		wantMessage string
	}{
		{name: caseMissingLabel, update: func(args map[string]any) { delete(args, managedServiceLabelParam) }, wantMessage: errLabelRequired},
		{name: caseBlankLabelImageShareGroupToken, update: func(args map[string]any) { args[monitorAlertDefinitionLabelParam] = blankString }, wantMessage: errLabelNonEmpty},
		{name: caseNumericLabel, update: func(args map[string]any) { args[monitorAlertDefinitionLabelParam] = 123 }, wantMessage: errLabelNonEmpty},
		{name: "invalid label pattern", update: func(args map[string]any) { args[monitorAlertDefinitionLabelParam] = "-bad" }, wantMessage: "label must start and end with an alphanumeric character and contain only alphanumeric characters, hyphens, underscores, or periods"},
		{name: caseMissingRegion, update: func(args map[string]any) { delete(args, keySupportTicketRegion) }, wantMessage: "region is required"},
		{name: "blank region", update: func(args map[string]any) { args[keySupportTicketRegion] = blankString }, wantMessage: errPlacementGroupRegionBlank},
		{name: caseMissingType, update: func(args map[string]any) { delete(args, keyPlacementGroupTypeJSON) }, wantMessage: "placement_group_type is required"},
		{name: "numeric type", update: func(args map[string]any) { args[keyPlacementGroupTypeJSON] = 123 }, wantMessage: "placement_group_type must be a non-empty string"},
		{name: caseInvalidType, update: func(args map[string]any) { args[keyPlacementGroupTypeJSON] = "affinity:local" }, wantMessage: "placement_group_type must be anti_affinity:local"},
		{name: "missing policy", update: func(args map[string]any) { delete(args, keyPlacementGroupPolicyJSON) }, wantMessage: "placement_group_policy is required"},
		{name: "blank policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = blankString }, wantMessage: "placement_group_policy must be a non-empty string"},
		{name: "numeric policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = 123 }, wantMessage: "placement_group_policy must be a non-empty string"},
		{name: "invalid policy", update: func(args map[string]any) { args[keyPlacementGroupPolicyJSON] = "eventual" }, wantMessage: "placement_group_policy must be one of: flexible, strict"},
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
			_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

			args := placementGroupCreateArgs()
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

func TestLinodePlacementGroupCreateToolDryRunReturnsPreviewWithoutClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

	args := placementGroupCreateArgs()
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

	assertDryRunRequest(t, body, "POST", "/placement/groups")

	if !strings.Contains(textContent.Text, "side_effects") {
		t.Errorf("textContent.Text does not contain %v", "side_effects")
	}

	if !strings.Contains(textContent.Text, "will be created in region") {
		t.Errorf("textContent.Text does not contain %v", "will be created in region")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodePlacementGroupCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups)
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
	_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupCreateArgs()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create placement group") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create placement group")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodePlacementGroupCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcPlacementGroups {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreatePlacementGroupRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label != placementGroupCreateLabel {
			t.Errorf("got.Label = %v, want %v", got.Label, placementGroupCreateLabel)
		}

		if got.Region != placementGroupCreateRegion {
			t.Errorf("got.Region = %v, want %v", got.Region, placementGroupCreateRegion)
		}

		if got.PlacementGroupType != placementGroupType {
			t.Errorf("got.PlacementGroupType = %v, want %v", got.PlacementGroupType, placementGroupType)
		}

		if got.PlacementGroupPolicy != placementGroupCreatePolicy {
			t.Errorf("got.PlacementGroupPolicy = %v, want %v", got.PlacementGroupPolicy, placementGroupCreatePolicy)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PlacementGroup{ID: 123, Label: placementGroupCreateLabel, Region: placementGroupCreateRegion, PlacementGroupType: placementGroupType, PlacementGroupPolicy: placementGroupCreatePolicy, IsCompliant: true}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodePlacementGroupCreateTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, placementGroupCreateArgs()))
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

	if !strings.Contains(textContent.Text, placementGroupCreateLabel) {
		t.Errorf("textContent.Text does not contain %v", placementGroupCreateLabel)
	}

	if !strings.Contains(textContent.Text, placementGroupCreateRegion) {
		t.Errorf("textContent.Text does not contain %v", placementGroupCreateRegion)
	}
}

func placementGroupCreateArgs() map[string]any {
	return map[string]any{
		monitorAlertDefinitionLabelParam: placementGroupCreateLabel,
		keySupportTicketRegion:           placementGroupCreateRegion,
		keyPlacementGroupTypeJSON:        placementGroupType,
		keyPlacementGroupPolicyJSON:      placementGroupCreatePolicy,
		keyConfirm:                       true,
	}
}
