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
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	labelAssignedInstanceFirewall    = "assigned-instance-firewall"
	errStandardPageSizeRange         = "page_size must be an integer from 25 through 500"
	caseInvalidInstanceFirewallsPage = "invalid page"
	errInstanceFirewallsPageMin      = "page must be an integer greater than or equal to 1"
)

func TestLinodeInstanceFirewallListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceFirewallListTool(cfg)

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

	if !strings.Contains(string(tool.RawInputSchema), keyLinodeID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeID)
	}

	if !strings.Contains(string(tool.RawInputSchema), "page") {
		t.Errorf("tool.RawInputSchema missing key %v", "page")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyPageSize) {
		t.Errorf("tool.RawInputSchema missing key %v", keyPageSize)
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
	_, _, handler := gentools.NewLinodeInstanceFirewallListTool(cfg)

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

	firewalls := []linode.Firewall{{ID: 456, Label: labelWebFirewall, Status: statusEnabled}}

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
	_, _, srvHandler := gentools.NewLinodeInstanceFirewallListTool(srvCfg)

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
	_, _, srvHandler := gentools.NewLinodeInstanceFirewallListTool(srvCfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeInstanceInterfaceFirewallsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeInstanceInterfaceFirewallListTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyLinodeID, keyInterfaceID} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
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
	_, _, handler := gentools.NewLinodeInstanceInterfaceFirewallListTool(cfg)

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

	firewalls := []linode.Firewall{{ID: 789, Label: labelAssignedInstanceFirewall, Status: statusEnabled}}

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
	_, _, srvHandler := gentools.NewLinodeInstanceInterfaceFirewallListTool(srvCfg)

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
	_, _, srvHandler := gentools.NewLinodeInstanceInterfaceFirewallListTool(srvCfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve items") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve items")
	}
}
