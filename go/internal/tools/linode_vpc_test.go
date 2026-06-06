package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// End-to-end verification of VPC listing and filtering.
func TestLinodeVPCsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeVPCListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vpcs := []linode.VPC{
			{ID: 1, Label: labelProdVPC, Region: regionUSEast, Description: "Production VPC"},
			{ID: 2, Label: "dev-vpc", Region: regionEUWest, Description: "Development VPC"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: vpcs, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelProdVPC, "response should contain prod-vpc")
		expectContains(t, textContent.Text, "dev-vpc", "response should contain dev-vpc")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		vpcs := []linode.VPC{
			{ID: 1, Label: labelProdVPC, Region: regionUSEast},
			{ID: 2, Label: "dev-vpc", Region: regionEUWest},
			{ID: 3, Label: "staging-prod", Region: regionUSWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: vpcs, keyPage: 1, keyPages: 1, keyResults: 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeVPCListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLabel: "prod"})
		result, err := srvHandler(t.Context(), req)

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelProdVPC, "response should contain prod-vpc")
		expectContains(t, textContent.Text, "staging-prod", "response should contain staging-prod")
		expectNotContains(t, textContent.Text, "dev-vpc", "response should not contain dev-vpc")
	})
}

// End-to-end verification of VPC get workflow.
func TestLinodeVPCGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 123, Label: labelProdVPC, Region: regionUSEast, Description: "Production VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelProdVPC, "response should contain VPC label")
		expectContains(t, textContent.Text, regionUSEast, "response should contain VPC region")
	})
}

// TestLinodeVPCIPsListTool verifies the VPC IPs list tool (all VPCs)
// registers correctly and returns VPC IP address data.
func TestLinodeVPCIPsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeVPCIPsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_ips_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
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
			checkEqual(t, "/vpcs/ips", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: ips, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "10.0.0.1", "response should contain first IP")
		expectContains(t, textContent.Text, "10.0.1.1", "response should contain second IP")
	})
}

// TestLinodeVPCIPListTool verifies the VPC IP list tool (single VPC)
// registers correctly, validates vpc_id, and returns IP data for a specific VPC.
func TestLinodeVPCIPListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCIPListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_ip_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingVPCID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errVPCIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		addr := "10.0.0.5"
		ips := []linode.VPCIP{
			{Address: &addr, VPCID: 456, SubnetID: 20, Region: regionUSEast, Active: true},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/456/ips", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: ips, keyPage: 1, keyPages: 1, keyResults: 1,
			}), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "10.0.0.5", "response should contain IP address")
	})
}

// TestLinodeVPCSubnetsListTool verifies the VPC subnets list tool
// registers correctly, validates vpc_id, and returns subnet data.
func TestLinodeVPCSubnetsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_subnet_list", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

	t.Run(caseMissingVPCID, func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectTrue(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, errVPCIDRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		subnets := []linode.VPCSubnet{
			{ID: 10, Label: labelWebSubnet, IPv4: cidrV4},
			{ID: 11, Label: "db-subnet", IPv4: "10.0.1.0/24"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123/subnets", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: subnets, keyPage: 1, keyPages: 1, keyResults: 2,
			}), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelWebSubnet, "response should contain web-subnet")
		expectContains(t, textContent.Text, "db-subnet", "response should contain db-subnet")
	})
}

// TestLinodeVPCSubnetGetTool verifies the VPC subnet get tool
// registers correctly, validates required fields, and retrieves subnet details.
func TestLinodeVPCSubnetGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_subnet_get", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 10, Label: labelWebSubnet, IPv4: cidrV4}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelWebSubnet, "response should contain subnet label")
		expectContains(t, textContent.Text, cidrV4, "response should contain subnet CIDR")
	})
}

// End-to-end verification of VPC creation workflow.
func TestLinodeVPCCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_create", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		expectContains(t, props, "label", "schema should include label")
		expectContains(t, props, "region", "schema should include region")
		expectContains(t, props, "confirm", "schema should include confirm")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 999, Label: labelTestVPC, Region: regionUSEast, Description: "Test VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelTestVPC, "response should contain VPC label")
		expectContains(t, textContent.Text, "999", "response should contain VPC ID")
	})
}

// TestLinodeVPCUpdateTool verifies the VPC update tool
// registers correctly, validates required fields, and updates VPCs.
func TestLinodeVPCUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_update", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContains(t, props, "vpc_id", "schema should include vpc_id")
		expectContains(t, props, "label", "schema should include label")
		expectContains(t, props, "confirm", "schema should include confirm")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 123, Label: "updated-vpc", Region: regionUSEast, Description: "Updated VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeVPCDeleteTool verifies the VPC delete tool
// registers correctly, validates required fields, and deletes VPCs.
func TestLinodeVPCDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// Dry-run coverage for VPC delete. Kept in a sibling function so the
// main test's subtest count stays under maintidx's threshold.
func TestLinodeVPCDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCDeleteTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, "dry_run",
			"schema must advertise the dry_run boolean to the model")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32
		var sawDelete atomic.Bool

		vpcBody := `{"id":123,"label":"prod-vpc","region":"us-east","subnets":[]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			if r.Method == http.MethodDelete {
				sawDelete.Store(true)
			}

			if r.Method != http.MethodGet {
				t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.Header().Set("Content-Type", "application/json")

			if r.URL.Path == "/vpcs/123" {
				_, writeErr := w.Write([]byte(vpcBody))
				checkNoError(t, writeErr, "writing VPC response should not fail")

				return
			}

			// The Tier A walk also lists subnets; an empty page keeps this
			// subtest on the no-mutation and preview-shape contract.
			_, writeErr := w.Write([]byte(`{}`))
			checkNoError(t, writeErr, "writing subnet list response should not fail")
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

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, true, body[keyDryRun])
		checkEqual(t, "linode_vpc_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		expectTrue(t, isWouldObject)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/vpcs/123", would["path"])

		expectTrue(t, requestCount.Load() > 0, "dry_run must read state")
		expectFalse(t, sawDelete.Load(), "dry_run must never issue a DELETE")
	})

	t.Run("does not require confirm", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"id":123,"label":"prod-vpc"}`))
			checkNoError(t, writeErr, "writing VPC response should not fail")
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

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError,
			"dry_run without confirm must succeed; confirm only gates real execution")
	})

	t.Run("still validates vpc_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{keyDryRun: true})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, errVPCIDRequired)
	})
}

// End-to-end verification of VPC subnet creation workflow.
func TestLinodeVPCSubnetCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_subnet_create", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContains(t, props, "vpc_id", "schema should include vpc_id")
		expectContains(t, props, "label", "schema should include label")
		expectContains(t, props, keyIPv4, "schema should include ipv4")
		expectContains(t, props, "confirm", "schema should include confirm")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 50, Label: labelWebSubnet, IPv4: cidrV4}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123/subnets", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, labelWebSubnet, "response should contain subnet label")
		expectContains(t, textContent.Text, "50", "response should contain subnet ID")
	})
}

// End-to-end verification of VPC subnet update workflow.
func TestLinodeVPCSubnetUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_subnet_update", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		expectContains(t, props, "vpc_id", "schema should include vpc_id")
		expectContains(t, props, "subnet_id", "schema should include subnet_id")
		expectContains(t, props, "label", "schema should include label")
		expectContains(t, props, "confirm", "schema should include confirm")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 10, Label: labelUpdatedSubnet, IPv4: cidrV4}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeVPCSubnetDeleteTool verifies the VPC subnet delete tool
// registers correctly, validates required fields, and deletes subnets.
func TestLinodeVPCSubnetDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		checkEqual(t, "linode_vpc_subnet_delete", tool.Name, "tool name should match")
		expectNotEmpty(t, tool.Description, "tool should have a description")
		expectNotNil(t, handler, "handler should not be nil")
		expectContains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

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
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			expectTrue(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
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

		expectNoError(t, err, "handler should not return Go error")
		expectNotNil(t, result, "handler should return a result")
		expectFalse(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok, "content should be TextContent")
		expectContains(t, textContent.Text, "deleted", "response should confirm deletion")
	})
}

// Dry-run coverage for VPC subnet delete. Kept in a sibling function so
// the main test's subtest count stays under maintidx's threshold.
func TestLinodeVPCSubnetDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, "dry_run",
			"schema must advertise the dry_run boolean to the model")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32
		var sawDelete atomic.Bool

		subnetBody := `{"id":10,"label":"web-subnet","ipv4":"10.0.0.0/24","linodes":[]}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			if r.Method == http.MethodDelete {
				sawDelete.Store(true)
			}
			checkEqual(t, "/vpcs/123/subnets/10", r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, writeErr := w.Write([]byte(subnetBody))
				checkNoError(t, writeErr, "writing subnet response should not fail")

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

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		expectTrue(t, isText)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		checkEqual(t, true, body[keyDryRun])
		checkEqual(t, "linode_vpc_subnet_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		expectTrue(t, isWouldObject)
		checkEqual(t, "DELETE", would["method"])
		checkEqual(t, "/vpcs/123/subnets/10", would["path"])

		checkEqual(t, int32(1), requestCount.Load(),
			"dry_run must only issue a single GET")
		expectFalse(t, sawDelete.Load(), "dry_run must never issue DELETE")
	})

	t.Run("does not require confirm", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{"id":10,"label":"web-subnet"}`))
			checkNoError(t, writeErr, "writing subnet response should not fail")
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

		expectNoError(t, err)
		expectNotNil(t, result)
		expectFalse(t, result.IsError,
			"dry_run without confirm must succeed; confirm only gates real execution")
	})

	t.Run("still validates vpc_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keySubnetID: float64(10),
			keyDryRun:   true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, errVPCIDRequired)
	})

	t.Run("still validates subnet_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyDryRun: true,
		})
		result, err := handler(t.Context(), req)

		expectNoError(t, err)
		expectNotNil(t, result)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, errSubnetIDRequired)
	})
}
