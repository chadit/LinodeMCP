package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

const (
	keyPlacementGroupID         = "group_id"
	placementGroupIDError       = "group_id must be a positive integer"
	placementGroupIDRequired    = "group_id is required"
	placementGroupLabel         = "PG_Miami_failover"
	placementGroupRegion        = "us-mia"
	placementGroupTypeLocal     = "anti_affinity:local"
	placementGroupPolicy        = "strict"
	keyPlacementIsCompliant     = "is_compliant"
	keyPlacementGroupTypeJSON   = "placement_group_type"
	keyPlacementGroupPolicyJSON = "placement_group_policy"
	caseMissingGroupID          = "missing group id"
	caseSlashGroupID            = "slash group id"
	caseQueryGroupID            = "query group id"
	caseTraversalGroupID        = "traversal group id"
	placementGroupSlashValue    = "528/529"
	placementGroupQueryValue    = "528?x=1"
	keyPlacementMembers         = "members"
)

func TestLinodePlacementGroupGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodePlacementGroupGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_placement_group_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_placement_group_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability.String() != "CapRead" {
		t.Errorf("capability.String() = %v, want %v", capability.String(), "CapRead")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodePlacementGroupGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodePlacementGroupGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingGroupID, args: map[string]any{}, wantContains: placementGroupIDRequired},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber}, wantContains: placementGroupIDError},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0"}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestLinodePlacementGroupGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcPlacementGroups528 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
			keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
			keyPlacementIsCompliant: true, keyPlacementMembers: []map[string]any{{keyLinodeID: 123, keyPlacementIsCompliant: true}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := gentools.NewLinodePlacementGroupGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: float64(528)})

	result, err := srvHandler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, placementGroupLabel) {
		t.Errorf("textContent.Text does not contain %v", placementGroupLabel)
	}

	if !strings.Contains(textContent.Text, placementGroupRegion) {
		t.Errorf("textContent.Text does not contain %v", placementGroupRegion)
	}
}
