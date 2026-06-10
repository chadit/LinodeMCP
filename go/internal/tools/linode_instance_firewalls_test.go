package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyFirewallIDs                    = "firewall_ids"
	labelAssignedInstanceFirewall     = "assigned-instance-firewall"
	errInstanceFirewallsPageSizeRange = "page_size must be an integer from 25 through 500"
	errInstanceFirewallsArray         = "firewall_ids must be an array"
	errInstanceFirewallsEntries       = "firewall_ids entries must be positive integers"
	caseInvalidInstanceFirewallsPage  = "invalid page"
	errInstanceFirewallsPageMin       = "page must be an integer greater than or equal to 1"
)

func TestLinodeInstanceFirewallListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceFirewallListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_firewall_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_firewall_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if _, ok := tool.InputSchema.Properties[keyLinodeID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyLinodeID)
	}

	if _, ok := tool.InputSchema.Properties["page"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "page")
	}

	if _, ok := tool.InputSchema.Properties[keyPageSize]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyPageSize)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}
}

func TestLinodeInstanceFirewallListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceFirewallListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: errLinodeIDMin},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
		{name: caseInvalidInstanceFirewallsPage, args: map[string]any{keyLinodeID: float64(123), keyPage: float64(0)}, wantContains: errInstanceFirewallsPageMin},
		{name: caseInvalidPageSizeLow, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(10)}, wantContains: errPageSizeRange},
		{name: caseInvalidPageSizeHigh, args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(501)}, wantContains: errPageSizeRange},
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

func TestLinodeInstanceFirewallListToolSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: labelWebFirewall, Status: "enabled"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Firewalls {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Firewalls)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "50" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "50")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeInstanceFirewallListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})

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

	if !strings.Contains(textContent.Text, labelWebFirewall) {
		t.Errorf("textContent.Text does not contain %v", labelWebFirewall)
	}

	if !strings.Contains(textContent.Text, "456") {
		t.Errorf("textContent.Text does not contain %v", "456")
	}
}

func TestLinodeInstanceFirewallListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
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
	_, _, srvHandler := tools.NewLinodeInstanceFirewallListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list firewalls for instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list firewalls for instance 123")
	}
}

func TestLinodeInstanceInterfaceFirewallsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceFirewallsListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_interface_firewall_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_interface_firewall_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	for _, key := range []string{keyLinodeID, keyInterfaceID} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}
}

func TestLinodeInstanceInterfaceFirewallsListToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeInstanceInterfaceFirewallsListTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123)}, wantContains: "interface_id is required"},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue}, wantContains: errInterfaceIDInteger},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue}, wantContains: errInterfaceIDInteger},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue}, wantContains: errInterfaceIDInteger},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(-1)}, wantContains: errInterfaceIDMinOne},
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

func TestLinodeInstanceInterfaceFirewallsListToolSuccess(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 789, Label: labelAssignedInstanceFirewall, Status: "enabled"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/interfaces/456/firewalls" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/interfaces/456/firewalls")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
			Data:    firewalls,
			Page:    1,
			Pages:   1,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceFirewallsListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})

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

	if !strings.Contains(textContent.Text, labelAssignedInstanceFirewall) {
		t.Errorf("textContent.Text does not contain %v", labelAssignedInstanceFirewall)
	}

	if !strings.Contains(textContent.Text, "789") {
		t.Errorf("textContent.Text does not contain %v", "789")
	}
}

func TestLinodeInstanceInterfaceFirewallsListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeInstanceInterfaceFirewallsListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to list firewalls for interface 456 on instance 123") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to list firewalls for interface 456 on instance 123")
	}
}

func TestLinodeInstanceFirewallsUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeInstanceFirewallsUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_instance_firewall_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_firewall_update")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyLinodeID]; !ok {
		t.Errorf("props missing key %v", keyLinodeID)
	}

	if _, ok := props[keyFirewallIDs]; !ok {
		t.Errorf("props missing key %v", keyFirewallIDs)
	}

	if _, ok := props[keyPage]; !ok {
		t.Errorf("props missing key %v", keyPage)
	}

	if _, ok := props[keyPageSize]; !ok {
		t.Errorf("props missing key %v", keyPageSize)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}
}

func TestLinodeInstanceFirewallsUpdateToolConfirm(t *testing.T) {
	t.Parallel()

	confirmTests := []struct {
		name    string
		confirm any
	}{
		{name: "missing confirm"},
		{name: caseFalseConfirm, confirm: false},
		{name: caseStringConfirm, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: float64(1)},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}}
			if tt.confirm != nil {
				args[keyConfirm] = tt.confirm
			}

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, srvHandler := tools.NewLinodeInstanceFirewallsUpdateTool(srvCfg)

			result, err := srvHandler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeInstanceFirewallsUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeInstanceFirewallsUpdateTool(cfg)

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing linode id", args: map[string]any{keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDRequired},
		{name: "separator instance id", args: map[string]any{keyLinodeID: pathSeparatorValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "query linode id", args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "traversal instance id", args: map[string]any{keyLinodeID: pathTraversalValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "fractional instance id", args: map[string]any{keyLinodeID: float64(123.9), keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "missing firewall ids", args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, want: "firewall_ids is required"},
		{name: "invalid firewall ids", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: "456", keyConfirm: true}, want: errInstanceFirewallsArray},
		{name: "invalid firewall id entry", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(0)}, keyConfirm: true}, want: errInstanceFirewallsEntries},
		{name: caseInvalidInstanceFirewallsPage, args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}, keyPage: float64(0), keyConfirm: true}, want: errInstanceFirewallsPageMin},
		{name: "invalid page size", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}, keyPageSize: float64(501), keyConfirm: true}, want: errInstanceFirewallsPageSizeRange},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.want) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.want)
			}
		})
	}
}

func TestLinodeInstanceFirewallsUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: labelAssignedInstanceFirewall, Status: statusEnabled}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Firewalls {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Firewalls)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		var body linode.UpdateInstanceFirewallsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body.FirewallIDs, []int{456, 789}) {
			t.Errorf("body.FirewallIDs = %v, want %v", body.FirewallIDs, []int{456, 789})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
			Data:    firewalls,
			Page:    2,
			Pages:   4,
			Results: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, srvHandler := tools.NewLinodeInstanceFirewallsUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLinodeID:    float64(123),
		keyFirewallIDs: []any{float64(456), float64(789)},
		keyPage:        float64(2),
		keyPageSize:    float64(25),
		keyConfirm:     true,
	})

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

	if !strings.Contains(textContent.Text, labelAssignedInstanceFirewall) {
		t.Errorf("textContent.Text does not contain %v", labelAssignedInstanceFirewall)
	}
}
