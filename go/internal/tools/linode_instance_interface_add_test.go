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
	toolLinodeInstanceInterfaceAdd = "linode_instance_interface_add"
	publicInterfaceAddJSON         = `{"public":{"ipv4":{"addresses":[{"address":"auto","primary":true}]}},"default_route":{"ipv4":true}}`
	vpcInterfaceAddJSON            = `{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"auto","primary":true,"nat_1_1_address":"auto"}],"ranges":[{"range":"/28"}]}},"default_route":{"ipv4":true},"firewall_id":321}`
	vlanInterfaceAddJSON           = `{"vlan":{"vlan_label":"backend","ipam_address":"10.0.0.1/24"}}`
)

func TestLinodeInstanceInterfaceAddToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceAddTool(cfg)

	t.Parallel()

	if tool.Name != toolLinodeInstanceInterfaceAdd {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolLinodeInstanceInterfaceAdd)
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

	if _, ok := props[keyInterface]; !ok {
		t.Errorf("props missing key %v", keyInterface)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceInterfaceAddToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfaceAddTool(cfg)

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: map[string]any{keyIPv4: keyAddress}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: "unknown interface field", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"public":{},"typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: "missing interface type", args: map[string]any{keyLinodeID: float64(123), keyInterface: jsonObjectEmpty, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "multiple interface types", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"public":{},"vlan":{"vlan_label":"backend"}}`, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "invalid vpc subnet", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"vpc":{"subnet_id":0}}`, keyConfirm: true}, wantContains: "interface.vpc.subnet_id must be a positive integer"},
		{name: "blank vlan label", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"vlan":{"vlan_label":"  "}}`, keyConfirm: true}, wantContains: "interface.vlan.vlan_label is required"},
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

func TestLinodeInstanceInterfaceAddToolSuccessfulPublicInterfaceCreation(t *testing.T) {
	t.Parallel()

	created := linode.InstanceInterface{ID: 1234, Public: &linode.InterfacePublicConfig{}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
		}

		var got linode.AddInstanceInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		switch {
		case got.Public == nil || got.Public.IPv4 == nil:
			t.Error("public interface IPv4 should be sent")
		case got.Public.IPv4.Addresses[0].Address != "auto":
			t.Errorf("got.Public.IPv4.Addresses[0].Address = %v, want %v", got.Public.IPv4.Addresses[0].Address, "auto")
		}

		switch {
		case got.DefaultRoute == nil || got.DefaultRoute.IPv4 == nil:
			t.Error("IPv4 default route should be sent")
		case !(*got.DefaultRoute.IPv4):
			t.Error("*got.DefaultRoute.IPv4 = false, want true")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceAddTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "1234") {
		t.Errorf("textContent.Text does not contain %v", "1234")
	}

	if !strings.Contains(textContent.Text, "Interface added to instance 123") {
		t.Errorf("textContent.Text does not contain %v", "Interface added to instance 123")
	}
}

func TestLinodeInstanceInterfaceAddToolAcceptsVpcAndVlanInterfaceBodies(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.InstanceInterface{ID: int(calls.Load())}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceAddTool(srvCfg)

	for _, body := range []string{vpcInterfaceAddJSON, vlanInterfaceAddJSON} {
		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterface: body, keyConfirm: true})

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
