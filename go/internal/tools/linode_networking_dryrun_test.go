package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

func TestLinodeNodeBalancerDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/nodebalancers/888": NodeBalancer{ID: 888, Label: "prod-lb"},
		"/nodebalancers/888/configs": PaginatedResponse[NodeBalancerConfig]{
			Data: []NodeBalancerConfig{
				{ID: 10, Port: 80, Protocol: "http"},
				{ID: 11, Port: 443, Protocol: "https"},
			},
		},
	})

	_, _, handler := gentools.NewLinodeNodebalancerDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(888),
		keyDryRun:         true,
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

	if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_delete")
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

		if !reflect.DeepEqual(dep[tcKind], "nodebalancer_config") {
			t.Errorf("got %v, want %v", dep[tcKind], "nodebalancer_config")
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
