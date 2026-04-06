package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestLinodeVPCsListTool verifies the VPCs list tool
// registers correctly, returns VPC data, and supports label filtering.
//
// Workflow:
//  1. Definition: Verify tool name, description, and handler
//  2. Success: List VPCs through mock API and verify response
//  3. FilterByLabel: Filter VPCs by label substring
//
// Expected Behavior:
//   - Tool registers as "linode_vpcs_list" with a valid handler
//   - Successful list returns all VPC names in the response
//   - Label filter returns only matching VPCs
//
// Purpose: End-to-end verification of VPC listing and filtering.
func TestLinodeVPCsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpcs_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vpcs := []linode.VPC{
			{ID: 1, Label: "prod-vpc", Region: "us-east", Description: "Production VPC"},
			{ID: 2, Label: "dev-vpc", Region: "eu-west", Description: "Development VPC"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": vpcs, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-vpc", "response should contain prod-vpc")
		assert.Contains(t, textContent.Text, "dev-vpc", "response should contain dev-vpc")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		vpcs := []linode.VPC{
			{ID: 1, Label: "prod-vpc", Region: "us-east"},
			{ID: 2, Label: "dev-vpc", Region: "eu-west"},
			{ID: 3, Label: "staging-prod", Region: "us-west"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": vpcs, "page": 1, "pages": 1, "results": 3,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"label": "prod"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-vpc", "response should contain prod-vpc")
		assert.Contains(t, textContent.Text, "staging-prod", "response should contain staging-prod")
		assert.NotContains(t, textContent.Text, "dev-vpc", "response should not contain dev-vpc")
	})
}

// TestLinodeVPCGetTool verifies the VPC get tool
// registers correctly, validates required fields, and retrieves VPC details.
//
// Workflow:
//  1. Definition: Verify tool name, description, and handler
//  2. Validation: Missing or invalid vpc_id produces clear errors
//  3. Success: Get VPC through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_vpc_get" with required params
//   - Missing vpc_id returns descriptive error
//   - Invalid vpc_id returns descriptive error
//   - Successful get returns VPC details from API
//
// Purpose: End-to-end verification of VPC get workflow.
func TestLinodeVPCGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing vpc id", args: map[string]any{}, wantContains: "vpc_id is required"},
		{name: "invalid vpc id", args: map[string]any{"vpc_id": "not-a-number"}, wantContains: "vpc_id must be a valid integer"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 123, Label: "prod-vpc", Region: "us-east", Description: "Production VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "prod-vpc", "response should contain VPC label")
		assert.Contains(t, textContent.Text, "us-east", "response should contain VPC region")
	})
}

// TestLinodeVPCIPsListTool verifies the VPC IPs list tool (all VPCs)
// registers correctly and returns VPC IP address data.
func TestLinodeVPCIPsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCIPsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_ips_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		addr1 := "10.0.0.1"
		addr2 := "10.0.1.1"
		ips := []linode.VPCIP{
			{Address: &addr1, VPCID: 1, SubnetID: 10, Region: "us-east", Active: true},
			{Address: &addr2, VPCID: 1, SubnetID: 11, Region: "us-east", Active: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/ips", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": ips, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCIPsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "10.0.0.1", "response should contain first IP")
		assert.Contains(t, textContent.Text, "10.0.1.1", "response should contain second IP")
	})
}

// TestLinodeVPCIPListTool verifies the VPC IP list tool (single VPC)
// registers correctly, validates vpc_id, and returns IP data for a specific VPC.
func TestLinodeVPCIPListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCIPListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_ip_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing vpc id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "vpc_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		addr := "10.0.0.5"
		ips := []linode.VPCIP{
			{Address: &addr, VPCID: 456, SubnetID: 20, Region: "us-east", Active: true},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/456/ips", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": ips, "page": 1, "pages": 1, "results": 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCIPListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": "456"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "10.0.0.5", "response should contain IP address")
	})
}

// TestLinodeVPCSubnetsListTool verifies the VPC subnets list tool
// registers correctly, validates vpc_id, and returns subnet data.
func TestLinodeVPCSubnetsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCSubnetsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_subnets_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing vpc id", func(t *testing.T) {
		t.Parallel()
		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "vpc_id is required")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		subnets := []linode.VPCSubnet{
			{ID: 10, Label: "web-subnet", IPv4: "10.0.0.0/24"},
			{ID: 11, Label: "db-subnet", IPv4: "10.0.1.0/24"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123/subnets", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": subnets, "page": 1, "pages": 1, "results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCSubnetsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": "123"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "web-subnet", "response should contain web-subnet")
		assert.Contains(t, textContent.Text, "db-subnet", "response should contain db-subnet")
	})
}

// TestLinodeVPCSubnetGetTool verifies the VPC subnet get tool
// registers correctly, validates required fields, and retrieves subnet details.
func TestLinodeVPCSubnetGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_subnet_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing vpc id", args: map[string]any{"subnet_id": "10"}, wantContains: "vpc_id is required"},
		{name: "missing subnet id", args: map[string]any{"vpc_id": "123"}, wantContains: "subnet_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 10, Label: "web-subnet", IPv4: "10.0.0.0/24"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCSubnetGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": "123", "subnet_id": "10"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "web-subnet", "response should contain subnet label")
		assert.Contains(t, textContent.Text, "10.0.0.0/24", "response should contain subnet CIDR")
	})
}

// TestLinodeVPCCreateTool verifies the VPC creation tool
// registers correctly, validates required fields, and creates VPCs.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, label, or region returns descriptive error
//  3. Success: Create VPC through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_vpc_create" with required params
//   - Missing required fields return descriptive errors
//   - Successful creation returns VPC details from API
//
// Purpose: End-to-end verification of VPC creation workflow.
func TestLinodeVPCCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "region", "schema should include region")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"label": "test-vpc", "region": "us-east"}, wantContains: "confirm=true"},
		{name: "missing label", args: map[string]any{"region": "us-east", "confirm": true}, wantContains: "label is required"},
		{name: "missing region", args: map[string]any{"label": "test-vpc", "confirm": true}, wantContains: "region is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 999, Label: "test-vpc", Region: "us-east", Description: "Test VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"label": "test-vpc", "region": "us-east", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "test-vpc", "response should contain VPC label")
		assert.Contains(t, textContent.Text, "999", "response should contain VPC ID")
	})
}

// TestLinodeVPCUpdateTool verifies the VPC update tool
// registers correctly, validates required fields, and updates VPCs.
func TestLinodeVPCUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "vpc_id", "schema should include vpc_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"vpc_id": float64(123), "label": "new-label"}, wantContains: "confirm=true"},
		{name: "missing vpc id", args: map[string]any{"label": "new-label", "confirm": true}, wantContains: "vpc_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		vpc := linode.VPC{ID: 123, Label: "updated-vpc", Region: "us-east", Description: "Updated VPC"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(vpc), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": float64(123), "label": "updated-vpc", "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeVPCDeleteTool verifies the VPC delete tool
// registers correctly, validates required fields, and deletes VPCs.
func TestLinodeVPCDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"vpc_id": float64(123)}, wantContains: "confirm=true"},
		{name: "missing vpc id", args: map[string]any{"confirm": true}, wantContains: "vpc_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"vpc_id": float64(123), "confirm": true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// TestLinodeVPCSubnetCreateTool verifies the VPC subnet creation tool
// registers correctly, validates required fields, and creates subnets.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, vpc_id, label, or ipv4 returns descriptive error
//  3. Success: Create subnet through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_vpc_subnet_create" with required params
//   - Missing required fields return descriptive errors
//   - Successful creation returns subnet details from API
//
// Purpose: End-to-end verification of VPC subnet creation workflow.
func TestLinodeVPCSubnetCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_subnet_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "vpc_id", "schema should include vpc_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "ipv4", "schema should include ipv4")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"vpc_id": float64(123), "label": "web-subnet", "ipv4": "10.0.0.0/24"}, wantContains: "confirm=true"},
		{name: "missing vpc id", args: map[string]any{"label": "web-subnet", "ipv4": "10.0.0.0/24", "confirm": true}, wantContains: "vpc_id is required"},
		{name: "missing label", args: map[string]any{"vpc_id": float64(123), "ipv4": "10.0.0.0/24", "confirm": true}, wantContains: "label is required"},
		{name: "missing ipv4", args: map[string]any{"vpc_id": float64(123), "label": "web-subnet", "confirm": true}, wantContains: "ipv4 is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 50, Label: "web-subnet", IPv4: "10.0.0.0/24"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123/subnets", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCSubnetCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"vpc_id": float64(123), "label": "web-subnet", "ipv4": "10.0.0.0/24", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "web-subnet", "response should contain subnet label")
		assert.Contains(t, textContent.Text, "50", "response should contain subnet ID")
	})
}

// TestLinodeVPCSubnetUpdateTool verifies the VPC subnet update tool
// registers correctly, validates required fields, and updates subnets.
//
// Workflow:
//  1. Definition: Verify tool name, description, and schema
//  2. Validation: Missing confirm, vpc_id, subnet_id, or label returns descriptive error
//  3. Success: Update subnet through mock API and verify response
//
// Expected Behavior:
//   - Tool registers as "linode_vpc_subnet_update" with required params
//   - Missing required fields return descriptive errors
//   - Successful update returns confirmation message
//
// Purpose: End-to-end verification of VPC subnet update workflow.
func TestLinodeVPCSubnetUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_subnet_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "vpc_id", "schema should include vpc_id")
		assert.Contains(t, props, "subnet_id", "schema should include subnet_id")
		assert.Contains(t, props, "label", "schema should include label")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"vpc_id": float64(123), "subnet_id": float64(10), "label": "updated-subnet"}, wantContains: "confirm=true"},
		{name: "missing vpc id", args: map[string]any{"subnet_id": float64(10), "label": "updated-subnet", "confirm": true}, wantContains: "vpc_id is required"},
		{name: "missing subnet id", args: map[string]any{"vpc_id": float64(123), "label": "updated-subnet", "confirm": true}, wantContains: "subnet_id is required"},
		{name: "missing label", args: map[string]any{"vpc_id": float64(123), "subnet_id": float64(10), "confirm": true}, wantContains: "label is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		subnet := linode.VPCSubnet{ID: 10, Label: "updated-subnet", IPv4: "10.0.0.0/24"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(subnet), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCSubnetUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"vpc_id": float64(123), "subnet_id": float64(10), "label": "updated-subnet", "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeVPCSubnetDeleteTool verifies the VPC subnet delete tool
// registers correctly, validates required fields, and deletes subnets.
func TestLinodeVPCSubnetDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_vpc_subnet_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing confirm", args: map[string]any{"vpc_id": float64(123), "subnet_id": float64(10)}, wantContains: "confirm=true"},
		{name: "missing vpc id", args: map[string]any{"subnet_id": float64(10), "confirm": true}, wantContains: "vpc_id is required"},
		{name: "missing subnet id", args: map[string]any{"vpc_id": float64(123), "confirm": true}, wantContains: "subnet_id is required"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("successful deletion", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeVPCSubnetDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"vpc_id": float64(123), "subnet_id": float64(10), "confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted", "response should confirm deletion")
	})
}
