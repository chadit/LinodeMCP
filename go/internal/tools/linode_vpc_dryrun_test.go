package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// TestLinodeVPCDeleteToolDryRunDependencies exercises the Phase 2 Tier A walk:
// each subnet is destroyed with the VPC (and its Linode interfaces detached),
// so subnets are surfaced as cascade_deleted dependencies.
func TestLinodeVPCDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/vpcs/888": VPC{ID: 888, Label: "prod-vpc"},
		"/vpcs/888/subnets": PaginatedResponse[VPCSubnet]{
			Data: []VPCSubnet{
				{ID: 1, Label: "subnet-a", Linodes: []VPCSubnetLinode{{ID: 456}}},
				{ID: 2, Label: "subnet-b"},
			},
		},
	})

	_, _, handler := gentools.NewLinodeVPCDeleteTool(cfg)

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
