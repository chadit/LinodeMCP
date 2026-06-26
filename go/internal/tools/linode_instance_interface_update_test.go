package tools_test

import (
	"encoding/json"
	"maps"
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
	toolLinodeInstanceInterfaceUpdate = "linode_instance_interface_update"
	keyInterfacePublic                = "public"
	keyInterfaceVPC                   = "vpc"
	keyInterfaceVLAN                  = "vlan"
	keyInterfaceDefaultRoute          = "default_route"
	keyAddresses                      = "addresses"
	keyVLANLabel                      = "vlan_label"
	valueAuto                         = "auto"
	primaryField                      = "primary"
	blankWhitespace                   = "  "
	errInterfaceAtLeastOneField       = "at least one of default_route, public, vpc, or vlan is required"
	errInterfaceExactlyOneType        = "exactly one of public, vpc, or vlan is required"
)

func publicInterfaceUpdateArgs() map[string]any {
	return map[string]any{
		keyInterfacePublic:       map[string]any{keyIPv4: map[string]any{keyAddresses: []any{map[string]any{keyAddress: valueAuto, primaryField: true}}}},
		keyInterfaceDefaultRoute: map[string]any{keyIPv4: true},
	}
}

func vpcInterfaceUpdateArgs() map[string]any {
	return map[string]any{
		keyInterfaceVPC: map[string]any{
			keySubnetID: float64(456),
			keyIPv4: map[string]any{
				keyAddresses: []any{map[string]any{keyAddress: valueAuto, primaryField: true, "nat_1_1_address": valueAuto}},
				"ranges":     []any{map[string]any{keyIPv6Range: "/28"}},
			},
		},
		keyInterfaceDefaultRoute: map[string]any{keyIPv4: true},
	}
}

func vlanInterfaceUpdateArgs() map[string]any {
	return map[string]any{
		keyInterfaceVLAN: map[string]any{keyVLANLabel: "backend", "ipam_address": "10.0.0.1/24"},
	}
}

// withInterfaceUpdatePath adds the linode/interface ids and confirm to a flat
// interface body so validation cases share one base argument set.
func withInterfaceUpdatePath(fields map[string]any) map[string]any {
	args := map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true}
	maps.Copy(args, fields)

	return args
}

func TestLinodeInstanceInterfaceUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}}}}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceUpdateTool(cfg)

	t.Parallel()

	if tool.Name != toolLinodeInstanceInterfaceUpdate {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeInstanceInterfaceUpdate)
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

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	for _, key := range []string{keyLinodeID, keyInterfaceID, keyInterfaceDefaultRoute, keyInterfacePublic, keyInterfaceVLAN, keyInterfaceVPC, keyConfirm} {
		if _, ok := props[key]; !ok {
			t.Errorf("props missing key %v", key)
		}
	}

	if _, ok := props[keyInterface]; ok {
		t.Errorf("props should not contain legacy key %v", keyInterface)
	}
}

func TestLinodeInstanceInterfaceUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeInstanceInterfaceUpdateTool(cfg)

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: "separator interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue, keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "query interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue, keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "traversal interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue, keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "missing interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "zero interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(0), keyInterfacePublic: map[string]any{}, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "no interface fields", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true}, wantContains: errInterfaceAtLeastOneField},
		{name: "non-object public", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: float64(1), keyConfirm: true}, wantContains: "public must be an object"},
		{name: "missing interface type", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfaceDefaultRoute: map[string]any{keyIPv4: true}, keyConfirm: true}, wantContains: errInterfaceExactlyOneType},
		{name: "multiple interface types", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfacePublic: map[string]any{}, keyInterfaceVLAN: map[string]any{"vlan_label": "backend"}, keyConfirm: true}, wantContains: errInterfaceExactlyOneType},
		{name: "invalid vpc subnet", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfaceVPC: map[string]any{keySubnetID: float64(0)}, keyConfirm: true}, wantContains: "vpc.subnet_id must be a positive integer"},
		{name: "blank vlan label", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterfaceVLAN: map[string]any{keyVLANLabel: blankWhitespace}, keyConfirm: true}, wantContains: "vlan.vlan_label is required"},
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

func TestLinodeInstanceInterfaceUpdateToolSuccessfulVpcInterfaceUpdate(t *testing.T) {
	t.Parallel()

	updated := linode.InstanceInterface{ID: 456, VPC: &linode.InterfaceVPCConfig{SubnetID: 789}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var got linode.UpdateInstanceInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.VPC == nil {
			t.Fatal("vpc interface should be sent")
		}

		if got.VPC.SubnetID != 456 {
			t.Errorf("got.VPC.SubnetID = %v, want %v", got.VPC.SubnetID, 456)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(updated); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

	req := createRequestWithArgs(t, withInterfaceUpdatePath(vpcInterfaceUpdateArgs()))

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

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}

	if !strings.Contains(textContent.Text, "Interface 456 updated on instance 123") {
		t.Errorf("textContent.Text does not contain %v", "Interface 456 updated on instance 123")
	}
}

func TestLinodeInstanceInterfaceUpdateToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

	req := createRequestWithArgs(t, withInterfaceUpdatePath(publicInterfaceUpdateArgs()))

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update interface 456 on instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update interface 456 on instance 123")
	}
}

func TestLinodeInstanceInterfaceUpdateToolAcceptsPublicAndVlanInterfaceBodies(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.InstanceInterface{ID: 456}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

	for _, body := range []map[string]any{publicInterfaceUpdateArgs(), vlanInterfaceUpdateArgs()} {
		req := createRequestWithArgs(t, withInterfaceUpdatePath(body))

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
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}
}
