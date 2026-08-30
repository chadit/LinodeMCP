package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

func TestLinodeFirewallDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/networking/firewalls/789": Firewall{ID: 789, Label: "prod-fw"},
		"/networking/firewalls/789/devices": PaginatedResponse[FirewallDevice]{
			Data: []FirewallDevice{
				{ID: 1, Entity: FirewallDeviceEntity{ID: 456, Type: keyDefaultFirewallLinode, Label: "fw-host"}},
				{ID: 2, Entity: FirewallDeviceEntity{ID: 99, Type: fwDeviceTypeNodeBalancer, Label: "fw-lb"}},
			},
		},
	})

	_, _, handler := gentools.NewLinodeFirewallDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyDryRun:     true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Errorf("len(deps) = %d, want %d", len(deps), 2)
	}

	kinds := make([]string, 0, len(deps))

	for _, entry := range deps {
		dep, depOK := entry.(map[string]any)
		if !depOK {
			t.Error("ok = false, want true")
		}

		kind, ok := dep[tcKind].(string)
		if !ok {
			t.Error("ok = false, want true")
		}

		kinds = append(kinds, kind)
	}

	gotElems1 := slices.Clone(kinds)
	wantElems1 := slices.Clone([]string{keyDefaultFirewallLinode, fwDeviceTypeNodeBalancer})

	slices.Sort(gotElems1)
	slices.Sort(wantElems1)

	if !slices.Equal(gotElems1, wantElems1) {
		t.Errorf("elements = %v, want %v (any order)", gotElems1, []string{keyDefaultFirewallLinode, fwDeviceTypeNodeBalancer})
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}

const fwDeviceTypeNodeBalancer = "nodebalancer"
