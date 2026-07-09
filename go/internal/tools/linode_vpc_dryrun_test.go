package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeVPCCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVPCCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeVPCCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  "vpc-01",
		keyRegion: regionUSEast,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/vpcs") {
		t.Errorf("got %v, want %v", would["path"], "/vpcs")
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "vpc-01") {
		t.Errorf("effect does not contain %v", "vpc-01")
	}

	if !strings.Contains(effect, regionUSEast) {
		t.Errorf("effect does not contain %v", regionUSEast)
	}
}

func TestLinodeVPCCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

func TestLinodeVPCUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/vpcs/123", linode.VPC{ID: 123, Label: "vpc"})
		_, _, handler := tools.NewLinodeVPCUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_vpc_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_vpc_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], tcVpcs123) {
			t.Errorf("got %v, want %v", would["path"], tcVpcs123)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, testRenamedLabel) {
			t.Errorf("effect does not contain %v", testRenamedLabel)
		}
	})

	t.Run("still validates vpc_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "vpc_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "vpc_id is required")
		}
	})
}

func TestLinodeVPCSubnetCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVPCSubnetCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeVPCSubnetCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCSubnetCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(123),
		keyLabel:  "subnet-01",
		keyIPv4:   cidrV4,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_subnet_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_subnet_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/vpcs/123/subnets") {
		t.Errorf("got %v, want %v", would["path"], "/vpcs/123/subnets")
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, "subnet-01") {
		t.Errorf("effect does not contain %v", "subnet-01")
	}

	if !strings.Contains(effect, cidrV4) {
		t.Errorf("effect does not contain %v", cidrV4)
	}
}

func TestLinodeVPCSubnetCreateToolDryRunStillValidatesVpcId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCSubnetCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  "subnet-01",
		keyIPv4:   cidrV4,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "vpc_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "vpc_id is required")
	}
}

func TestLinodeVPCSubnetUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCSubnetUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/vpcs/123/subnets/456", linode.VPCSubnet{ID: 456, Label: "subnet"})
		_, _, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:    float64(123),
			keySubnetID: float64(456),
			keyLabel:    testRenamedLabel,
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_vpc_subnet_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_vpc_subnet_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/vpcs/123/subnets/456") {
			t.Errorf("got %v, want %v", would["path"], "/vpcs/123/subnets/456")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates subnet_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "subnet_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "subnet_id is required")
		}
	})
}

// TestLinodeVPCDeleteToolDryRunDependencies exercises the Phase 2 Tier A walk:
// each subnet is destroyed with the VPC (and its Linode interfaces detached),
// so subnets are surfaced as cascade_deleted dependencies.
func TestLinodeVPCDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/vpcs/888": linode.VPC{ID: 888, Label: "prod-vpc"},
		"/vpcs/888/subnets": linode.PaginatedResponse[linode.VPCSubnet]{
			Data: []linode.VPCSubnet{
				{ID: 1, Label: "subnet-a", Linodes: []linode.VPCSubnetLinode{{ID: 456}}},
				{ID: 2, Label: "subnet-b"},
			},
		},
	})

	_, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(888),
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 2)
	}

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		if !gotMap {
			t.Fatal("gotMap = false, want true")
		}

		if !reflect.DeepEqual(dep[tcKind], "vpc_subnet") {
			t.Errorf("got %v, want %v", dep[tcKind], "vpc_subnet")
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("warnings is empty")
	}

	warning, gotString := warnings[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(warning, "1 Linode interface(s)") {
		t.Errorf("warning does not contain %v", "1 Linode interface(s)")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}

// TestLinodeVPCSubnetDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: Linodes with interfaces in the subnet are surfaced as detached
// dependencies, read from the subnet state (the parent VPC is fetched once).
func TestLinodeVPCSubnetDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/vpcs/888/subnets/777": linode.VPCSubnet{
			ID:      777,
			Label:   "app-subnet",
			Linodes: []linode.VPCSubnetLinode{{ID: 456}},
		},
		"/vpcs/888": linode.VPC{ID: 888, Label: "app-vpc"},
	})

	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVPCID:    float64(888),
		keySubnetID: float64(777),
		keyDryRun:   true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_subnet_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_subnet_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 1)
	}

	dep, gotMap := deps[0].(map[string]any)
	if !gotMap {
		t.Fatal("gotMap = false, want true")
	}

	for key, want := range map[string]any{
		tcKind:             tcInstance,
		tcAction:           "detached",
		keySupportTicketID: float64(456),
	} {
		if !reflect.DeepEqual(dep[key], want) {
			t.Errorf("dep[%v] = %v, want %v", key, dep[key], want)
		}
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
