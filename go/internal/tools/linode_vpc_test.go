package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// End-to-end verification of VPC listing and filtering.
func TestLinodeVPCsListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeVPCListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVPCsListToolSuccess(t *testing.T) {
	t.Parallel()

	vpcs := []linode.VPC{
		{ID: 1, Label: labelProdVPC, Region: regionUSEast, Description: "Production VPC"},
		{ID: 2, Label: "dev-vpc", Region: regionEUWest, Description: "Development VPC"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vpcs" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: vpcs, keyPage: 1, keyPages: 1, keyResults: 2,
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
	_, _, srvHandler := tools.NewLinodeVPCListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

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

	if !strings.Contains(textContent.Text, labelProdVPC) {
		t.Errorf("textContent.Text does not contain %v", labelProdVPC)
	}

	if !strings.Contains(textContent.Text, "dev-vpc") {
		t.Errorf("textContent.Text does not contain %v", "dev-vpc")
	}
}

func TestLinodeVPCsListToolFilterByLabel(t *testing.T) {
	t.Parallel()

	vpcs := []linode.VPC{
		{ID: 1, Label: labelProdVPC, Region: regionUSEast},
		{ID: 2, Label: "dev-vpc", Region: regionEUWest},
		{ID: 3, Label: "staging-prod", Region: regionUSWest},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: vpcs, keyPage: 1, keyPages: 1, keyResults: 3,
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
	_, _, srvHandler := tools.NewLinodeVPCListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: canRunEnvProd})

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

	if !strings.Contains(textContent.Text, labelProdVPC) {
		t.Errorf("textContent.Text does not contain %v", labelProdVPC)
	}

	if !strings.Contains(textContent.Text, "staging-prod") {
		t.Errorf("textContent.Text does not contain %v", "staging-prod")
	}

	if strings.Contains(textContent.Text, "dev-vpc") {
		t.Errorf("textContent.Text should not contain %v", "dev-vpc")
	}
}

// End-to-end verification of VPC get workflow.
func TestLinodeVPCGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVPCGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingVPCID, args: map[string]any{}, wantContains: errVPCIDRequired},
		{name: "invalid vpc id", args: map[string]any{keyVPCID: notANumber}, wantContains: "vpc_id must be a valid integer"},
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

func TestLinodeVPCGetToolSuccess(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{ID: 123, Label: labelProdVPC, Region: regionUSEast, Description: "Production VPC"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(vpc); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: "123"})

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

	if !strings.Contains(textContent.Text, labelProdVPC) {
		t.Errorf("textContent.Text does not contain %v", labelProdVPC)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}
}

// TestLinodeVPCIPsListTool verifies the VPC IPs list tool (all VPCs)
// registers correctly and returns VPC IP address data.
func TestLinodeVPCIPsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeVPCIPsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_vpc_ip_all_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_ip_all_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		addr1 := "10.0.0.1"
		addr2 := "10.0.1.1"
		ips := []linode.VPCIP{
			{Address: &addr1, VPCID: 1, SubnetID: 10, Region: regionUSEast, Active: true},
			{Address: &addr2, VPCID: 1, SubnetID: 11, Region: regionUSEast, Active: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/vpcs/ips" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs/ips")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData: ips, keyPage: 1, keyPages: 1, keyResults: 2,
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
		_, _, srvHandler := tools.NewLinodeVPCIPsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})

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

		if !strings.Contains(textContent.Text, "10.0.0.1") {
			t.Errorf("textContent.Text does not contain %v", "10.0.0.1")
		}

		if !strings.Contains(textContent.Text, "10.0.1.1") {
			t.Errorf("textContent.Text does not contain %v", "10.0.1.1")
		}
	})
}

// TestLinodeVPCIPListTool verifies the VPC IP list tool (single VPC)
// registers correctly, validates vpc_id, and returns IP data for a specific VPC.
func TestLinodeVPCIPListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCIPListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_ip_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_ip_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVPCIPListToolCaseMissingVPCID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCIPListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVPCIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVPCIDRequired)
	}
}

func TestLinodeVPCIPListToolSuccess(t *testing.T) {
	t.Parallel()

	addr := "10.0.0.5"
	ips := []linode.VPCIP{
		{Address: &addr, VPCID: 456, SubnetID: 20, Region: regionUSEast, Active: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vpcs/456/ips" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs/456/ips")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: ips, keyPage: 1, keyPages: 1, keyResults: 1,
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
	_, _, srvHandler := tools.NewLinodeVPCIPListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: "456"})

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

	if !strings.Contains(textContent.Text, "10.0.0.5") {
		t.Errorf("textContent.Text does not contain %v", "10.0.0.5")
	}
}

// TestLinodeVPCSubnetsListTool verifies the VPC subnets list tool
// registers correctly, validates vpc_id, and returns subnet data.
func TestLinodeVPCSubnetsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_subnet_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_subnet_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVPCSubnetsListToolCaseMissingVPCID(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCSubnetListTool(cfg)

	t.Parallel()
	req := createRequestWithArgs(t, map[string]any{})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVPCIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVPCIDRequired)
	}
}

func TestLinodeVPCSubnetsListToolSuccess(t *testing.T) {
	t.Parallel()

	subnets := []linode.VPCSubnet{
		{ID: 10, Label: labelWebSubnet, IPv4: cidrV4},
		{ID: 11, Label: "db-subnet", IPv4: "10.0.1.0/24"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vpcs/123/subnets" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs/123/subnets")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: subnets, keyPage: 1, keyPages: 1, keyResults: 2,
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
	_, _, srvHandler := tools.NewLinodeVPCSubnetListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: "123"})

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

	if !strings.Contains(textContent.Text, labelWebSubnet) {
		t.Errorf("textContent.Text does not contain %v", labelWebSubnet)
	}

	if !strings.Contains(textContent.Text, "db-subnet") {
		t.Errorf("textContent.Text does not contain %v", "db-subnet")
	}
}

// TestLinodeVPCSubnetGetTool verifies the VPC subnet get tool
// registers correctly, validates required fields, and retrieves subnet details.
func TestLinodeVPCSubnetGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_subnet_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_subnet_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVPCSubnetGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingVPCID, args: map[string]any{keySubnetID: "10"}, wantContains: errVPCIDRequired},
		{name: caseMissingSubnetID, args: map[string]any{keyVPCID: "123"}, wantContains: errSubnetIDRequired},
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

func TestLinodeVPCSubnetGetToolSuccess(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{ID: 10, Label: labelWebSubnet, IPv4: cidrV4}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123Subnets10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123Subnets10)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(subnet); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCSubnetGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: "123", keySubnetID: "10"})

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

	if !strings.Contains(textContent.Text, labelWebSubnet) {
		t.Errorf("textContent.Text does not contain %v", labelWebSubnet)
	}

	if !strings.Contains(textContent.Text, cidrV4) {
		t.Errorf("textContent.Text does not contain %v", cidrV4)
	}
}

// End-to-end verification of VPC creation workflow.
func TestLinodeVPCCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_create")
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

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyLabel) {
		t.Errorf("RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(rawSchema, keyRegion) {
		t.Errorf("RawInputSchema missing key %v", keyRegion)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeVPCCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLabel: labelTestVPC, keyRegion: regionUSEast}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast, keyConfirm: true}, wantContains: errLabelRequired},
		{name: caseMissingRegion, args: map[string]any{keyLabel: labelTestVPC, keyConfirm: true}, wantContains: errRegionRequired},
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

func TestLinodeVPCCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{ID: 999, Label: labelTestVPC, Region: regionUSEast, Description: "Test VPC"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vpcs" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(vpc); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyLabel: labelTestVPC, keyRegion: regionUSEast, keyConfirm: true})

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

	if !strings.Contains(textContent.Text, labelTestVPC) {
		t.Errorf("textContent.Text does not contain %v", labelTestVPC)
	}

	if !strings.Contains(textContent.Text, "999") {
		t.Errorf("textContent.Text does not contain %v", "999")
	}
}

// TestLinodeVPCUpdateTool verifies the VPC update tool
// registers correctly, validates required fields, and updates VPCs.
func TestLinodeVPCUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyVPCID) {
		t.Errorf("RawInputSchema missing key %v", keyVPCID)
	}

	if !strings.Contains(rawSchema, keyLabel) {
		t.Errorf("RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeVPCUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyVPCID: float64(123), keyLabel: labelNew}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVPCID, args: map[string]any{keyLabel: labelNew, keyConfirm: true}, wantContains: errVPCIDRequired},
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

func TestLinodeVPCUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{ID: 123, Label: "updated-vpc", Region: regionUSEast, Description: "Updated VPC"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(vpc); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: float64(123), keyLabel: "updated-vpc", keyConfirm: true})

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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// TestLinodeVPCDeleteTool verifies the VPC delete tool
// registers correctly, validates required fields, and deletes VPCs.
func TestLinodeVPCDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_delete")
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
}

func TestLinodeVPCDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyVPCID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVPCID, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errVPCIDRequired},
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

func TestLinodeVPCDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyVPCID: float64(123), keyConfirm: true, keyConfirmedDryRun: true})

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

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

// Dry-run coverage for VPC delete. Kept in a sibling function so the
// main test's subtest count stays under maintidx's threshold.
func TestLinodeVPCDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVPCDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeVPCDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	vpcBody := `{"id":123,"label":"prod-vpc","region":"us-east","subnets":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)

		if r.Method != http.MethodGet {
			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == tcVpcs123 {
			_, _ = w.Write([]byte(vpcBody))

			return
		}

		// The Tier A walk also lists subnets; an empty page keeps this
		// subtest on the no-mutation and preview-shape contract.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(123),
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcVpcs123) {
		t.Errorf("got %v, want %v", would["path"], tcVpcs123)
	}

	if len(methodsSeen) == 0 {
		t.Fatal("methodsSeen is empty")
	}

	if slices.Contains(methodsSeen, http.MethodDelete) {
		t.Errorf("methodsSeen should not contain %v", http.MethodDelete)
	}
}

func TestLinodeVPCDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"label":"prod-vpc"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(123),
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
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

func TestLinodeVPCDeleteToolDryRunStillValidatesVpcId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVPCIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVPCIDRequired)
	}
}

// End-to-end verification of VPC subnet creation workflow.
func TestLinodeVPCSubnetCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_subnet_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_subnet_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyVPCID) {
		t.Errorf("RawInputSchema missing key %v", keyVPCID)
	}

	if !strings.Contains(rawSchema, keyLabel) {
		t.Errorf("RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(rawSchema, keyIPv4) {
		t.Errorf("RawInputSchema missing key %v", keyIPv4)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeVPCSubnetCreateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyVPCID: float64(123), keyLabel: labelWebSubnet, keyIPv4: cidrV4}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVPCID, args: map[string]any{keyLabel: labelWebSubnet, keyIPv4: cidrV4, keyConfirm: true}, wantContains: errVPCIDRequired},
		{name: caseMissingLabel, args: map[string]any{keyVPCID: float64(123), keyIPv4: cidrV4, keyConfirm: true}, wantContains: errLabelRequired},
		{name: "missing ipv4", args: map[string]any{keyVPCID: float64(123), keyLabel: labelWebSubnet, keyConfirm: true}, wantContains: "ipv4 is required"},
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

func TestLinodeVPCSubnetCreateToolSuccessfulCreation(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{ID: 50, Label: labelWebSubnet, IPv4: cidrV4}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vpcs/123/subnets" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/vpcs/123/subnets")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(subnet); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCSubnetCreateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID: float64(123), keyLabel: labelWebSubnet, keyIPv4: cidrV4, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, labelWebSubnet) {
		t.Errorf("textContent.Text does not contain %v", labelWebSubnet)
	}

	if !strings.Contains(textContent.Text, "50") {
		t.Errorf("textContent.Text does not contain %v", "50")
	}
}

// End-to-end verification of VPC subnet update workflow.
func TestLinodeVPCSubnetUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_subnet_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_subnet_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyVPCID) {
		t.Errorf("RawInputSchema missing key %v", keyVPCID)
	}

	if !strings.Contains(rawSchema, keySubnetID) {
		t.Errorf("RawInputSchema missing key %v", keySubnetID)
	}

	if !strings.Contains(rawSchema, keyLabel) {
		t.Errorf("RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeVPCSubnetUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyVPCID: float64(123), keySubnetID: float64(10), keyLabel: labelUpdatedSubnet}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVPCID, args: map[string]any{keySubnetID: float64(10), keyLabel: labelUpdatedSubnet, keyConfirm: true}, wantContains: errVPCIDRequired},
		{name: caseMissingSubnetID, args: map[string]any{keyVPCID: float64(123), keyLabel: labelUpdatedSubnet, keyConfirm: true}, wantContains: errSubnetIDRequired},
		{name: caseMissingLabel, args: map[string]any{keyVPCID: float64(123), keySubnetID: float64(10), keyConfirm: true}, wantContains: errLabelRequired},
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

func TestLinodeVPCSubnetUpdateToolSuccessfulUpdate(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{ID: 10, Label: labelUpdatedSubnet, IPv4: cidrV4}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123Subnets10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123Subnets10)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(subnet); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCSubnetUpdateTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID: float64(123), keySubnetID: float64(10), keyLabel: labelUpdatedSubnet, keyConfirm: true,
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

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// TestLinodeVPCSubnetDeleteTool verifies the VPC subnet delete tool
// registers correctly, validates required fields, and deletes subnets.
func TestLinodeVPCSubnetDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_vpc_subnet_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_vpc_subnet_delete")
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
}

func TestLinodeVPCSubnetDeleteToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyVPCID: float64(123), keySubnetID: float64(10)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingVPCID, args: map[string]any{keySubnetID: float64(10), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errVPCIDRequired},
		{name: caseMissingSubnetID, args: map[string]any{keyVPCID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, wantContains: errSubnetIDRequired},
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

func TestLinodeVPCSubnetDeleteToolSuccessfulDeletion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcVpcs123Subnets10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123Subnets10)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeVPCSubnetDeleteTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID: float64(123), keySubnetID: float64(10), keyConfirm: true, keyConfirmedDryRun: true,
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

	if !strings.Contains(textContent.Text, "deleted") {
		t.Errorf("textContent.Text does not contain %v", "deleted")
	}
}

// Dry-run coverage for VPC subnet delete. Kept in a sibling function so
// the main test's subtest count stays under maintidx's threshold.
func TestLinodeVPCSubnetDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeVPCSubnetDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	subnetBody := `{"id":10,"label":"web-subnet","ipv4":"10.0.0.0/24","linodes":[]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcVpcs123Subnets10 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcVpcs123Subnets10)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(subnetBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID:    float64(123),
		keySubnetID: float64(10),
		keyDryRun:   true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_vpc_subnet_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_vpc_subnet_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcVpcs123Subnets10) {
		t.Errorf("got %v, want %v", would["path"], tcVpcs123Subnets10)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeVPCSubnetDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":10,"label":"web-subnet"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyVPCID:    float64(123),
		keySubnetID: float64(10),
		keyDryRun:   true,
	})

	result, err := handler(t.Context(), req)
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

func TestLinodeVPCSubnetDeleteToolDryRunStillValidatesVpcId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keySubnetID: float64(10),
		keyDryRun:   true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errVPCIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errVPCIDRequired)
	}
}

func TestLinodeVPCSubnetDeleteToolDryRunStillValidatesSubnetId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(123),
		keyDryRun: true,
	})

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errSubnetIDRequired) {
		t.Errorf("error text %q does not contain %q", text.Text, errSubnetIDRequired)
	}
}
