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
	toolLinodeInstanceInterfaceUpdate = "linode_instance_interface_update"
	publicInterfaceUpdateJSON         = `{"public":{"ipv4":{"addresses":[{"address":"auto","primary":true}]}},"default_route":{"ipv4":true}}`
	vpcInterfaceUpdateJSON            = `{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"auto","primary":true,"nat_1_1_address":"auto"}],"ranges":[{"range":"/28"}]}},"default_route":{"ipv4":true}}`
	vlanInterfaceUpdateJSON           = `{"vlan":{"vlan_label":"backend","ipam_address":"10.0.0.1/24"}}`
)

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
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyInterfaceID]; !ok {
		t.Errorf("props missing key %v", keyInterfaceID)
	}

	if _, ok := props[keyInterface]; !ok {
		t.Errorf("props missing key %v", keyInterface)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceInterfaceUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeInstanceInterfaceUpdateTool(cfg)

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: "separator interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "query interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "traversal interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "missing interface id", args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "zero interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(0), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: map[string]any{keyIPv4: keyAddress}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: "unknown interface field", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"public":{},"typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: "missing interface type", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: jsonObjectEmpty, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "multiple interface types", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"public":{},"vlan":{"vlan_label":"backend"}}`, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "invalid vpc subnet", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"vpc":{"subnet_id":0}}`, keyConfirm: true}, wantContains: "interface.vpc.subnet_id must be a positive integer"},
		{name: "blank vlan label", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"vlan":{"vlan_label":"  "}}`, keyConfirm: true}, wantContains: "interface.vlan.vlan_label is required"},
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

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: vpcInterfaceUpdateJSON, keyConfirm: true})

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

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true})

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

	for _, body := range []string{publicInterfaceUpdateJSON, vlanInterfaceUpdateJSON} {
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: body, keyConfirm: true})

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
