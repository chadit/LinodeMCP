package tools_test

import (
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
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	keyPlacementGroupID         = "group_id"
	placementGroupIDError       = "group_id must be an integer"
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
	tool, capability, handler := tools.NewLinodePlacementGroupGetTool(cfg)

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
	_, _, handler := tools.NewLinodePlacementGroupGetTool(cfg)

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
	_, _, srvHandler := tools.NewLinodePlacementGroupGetTool(srvCfg)

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

func TestLinodePlacementGroupDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodePlacementGroupDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_placement_group_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_placement_group_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability.String() != "CapDestroy" {
		t.Errorf("capability.String() = %v, want %v", capability.String(), "CapDestroy")
	}

	for _, key := range []string{keyPlacementGroupID, keyConfirm, keyDryRun} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	for _, key := range []string{keyPlacementGroupID, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodePlacementGroupDeleteToolValidation(t *testing.T) {
	t.Parallel()

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyPlacementGroupID: float64(528)}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyPlacementGroupID: float64(528), keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyPlacementGroupID: float64(528), keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyPlacementGroupID: float64(528), keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingGroupID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDRequired},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseSlashGroupID, args: map[string]any{keyPlacementGroupID: placementGroupSlashValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseQueryGroupID, args: map[string]any{keyPlacementGroupID: placementGroupQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: caseTraversalGroupID, args: map[string]any{keyPlacementGroupID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0", keyConfirm: true, keyConfirmedDryRun: true}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			srvCfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				},
			}
			_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

			result, err := srvHandler(t.Context(), createRequestWithArgs(t, tt.args))
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

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestLinodePlacementGroupDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcPlacementGroups528 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: float64(528), keyConfirm: true, keyConfirmedDryRun: true}))
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

	if !strings.Contains(textContent.Text, "deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "deleted successfully")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestLinodePlacementGroupDeleteToolDryRunPreviewsWithoutDeleting(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcPlacementGroups528 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcPlacementGroups528)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
			keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
			keyPlacementIsCompliant: true, keyPlacementMembers: []map[string]any{},
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
	_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: float64(528), keyDryRun: true}))
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

	if !strings.Contains(textContent.Text, "dry_run") {
		t.Errorf("textContent.Text does not contain %v", "dry_run")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodePlacementGroupDeleteToolDryRunSurfacesMemberLinodesAsDetachedDependencies(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
			keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
			keyPlacementIsCompliant: true,
			keyPlacementMembers:     []map[string]any{{keyLinodeID: 111}, {keyLinodeID: 222}},
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
	_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: float64(528), keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "detached") {
		t.Errorf("textContent.Text does not contain %v", "detached")
	}

	if !strings.Contains(textContent.Text, "111") {
		t.Errorf("textContent.Text does not contain %v", "111")
	}

	if !strings.Contains(textContent.Text, "222") {
		t.Errorf("textContent.Text does not contain %v", "222")
	}

	if !strings.Contains(textContent.Text, "detaches 2 Linode") {
		t.Errorf("textContent.Text does not contain %v", "detaches 2 Linode")
	}
}

func TestLinodePlacementGroupDeleteToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
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
	_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

	result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: float64(528), keyConfirm: true, keyConfirmedDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_placement_group_delete failed") {
		t.Errorf("error text %q does not contain %q", text.Text, "linode_placement_group_delete failed")
	}
}
