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

// =============================================================================
// VPCs List Tool Tests
// =============================================================================

func TestNewLinodeVPCsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCsListTool(cfg)

	assert.Equal(t, "linode_vpcs_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCsListTool_Success(t *testing.T) {
	t.Parallel()

	vpcs := []linode.VPC{
		{ID: 1, Label: "prod-vpc", Region: "us-east", Description: "Production VPC"},
		{ID: 2, Label: "dev-vpc", Region: "eu-west", Description: "Development VPC"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    vpcs,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-vpc")
	assert.Contains(t, textContent.Text, "dev-vpc")
}

func TestLinodeVPCsListTool_FilterByLabel(t *testing.T) {
	t.Parallel()

	vpcs := []linode.VPC{
		{ID: 1, Label: "prod-vpc", Region: "us-east"},
		{ID: 2, Label: "dev-vpc", Region: "eu-west"},
		{ID: 3, Label: "staging-prod", Region: "us-west"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    vpcs,
			"page":    1,
			"pages":   1,
			"results": 3,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "prod"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-vpc")
	assert.Contains(t, textContent.Text, "staging-prod")
	assert.NotContains(t, textContent.Text, "dev-vpc")
}

// =============================================================================
// VPC Get Tool Tests
// =============================================================================

func TestNewLinodeVPCGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCGetTool(cfg)

	assert.Equal(t, "linode_vpc_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCGetTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCGetTool_InvalidVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "not-a-number"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id must be a valid integer")
}

func TestLinodeVPCGetTool_Success(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{
		ID:          123,
		Label:       "prod-vpc",
		Region:      "us-east",
		Description: "Production VPC",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(vpc))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "prod-vpc")
	assert.Contains(t, textContent.Text, "us-east")
}

// =============================================================================
// VPC IPs List Tool Tests (All VPCs)
// =============================================================================

func TestNewLinodeVPCIPsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCIPsListTool(cfg)

	assert.Equal(t, "linode_vpc_ips_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCIPsListTool_Success(t *testing.T) {
	t.Parallel()

	addr1 := "10.0.0.1"
	addr2 := "10.0.1.1"
	ips := []linode.VPCIP{
		{Address: &addr1, VPCID: 1, SubnetID: 10, Region: "us-east", Active: true},
		{Address: &addr2, VPCID: 1, SubnetID: 11, Region: "us-east", Active: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/ips", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    ips,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCIPsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "10.0.0.1")
	assert.Contains(t, textContent.Text, "10.0.1.1")
}

// =============================================================================
// VPC IP List Tool Tests (Single VPC)
// =============================================================================

func TestNewLinodeVPCIPListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCIPListTool(cfg)

	assert.Equal(t, "linode_vpc_ip_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCIPListTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCIPListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCIPListTool_Success(t *testing.T) {
	t.Parallel()

	addr := "10.0.0.5"
	ips := []linode.VPCIP{
		{Address: &addr, VPCID: 456, SubnetID: 20, Region: "us-east", Active: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/456/ips", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    ips,
			"page":    1,
			"pages":   1,
			"results": 1,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCIPListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "456"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "10.0.0.5")
}

// =============================================================================
// VPC Subnets List Tool Tests
// =============================================================================

func TestNewLinodeVPCSubnetsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCSubnetsListTool(cfg)

	assert.Equal(t, "linode_vpc_subnets_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCSubnetsListTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCSubnetsListTool_Success(t *testing.T) {
	t.Parallel()

	subnets := []linode.VPCSubnet{
		{ID: 10, Label: "web-subnet", IPv4: "10.0.0.0/24"},
		{ID: 11, Label: "db-subnet", IPv4: "10.0.1.0/24"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123/subnets", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    subnets,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-subnet")
	assert.Contains(t, textContent.Text, "db-subnet")
}

// =============================================================================
// VPC Subnet Get Tool Tests
// =============================================================================

func TestNewLinodeVPCSubnetGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	assert.Equal(t, "linode_vpc_subnet_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVPCSubnetGetTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"subnet_id": "10"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCSubnetGetTool_MissingSubnetID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "subnet_id is required")
}

func TestLinodeVPCSubnetGetTool_Success(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{
		ID:    10,
		Label: "web-subnet",
		IPv4:  "10.0.0.0/24",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(subnet))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": "123", "subnet_id": "10"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-subnet")
	assert.Contains(t, textContent.Text, "10.0.0.0/24")
}

// =============================================================================
// VPC Create Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCCreateTool(cfg)

	assert.Equal(t, "linode_vpc_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVPCCreateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":  "test-vpc",
		"region": "us-east",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should require confirm=true")
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeVPCCreateTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "test-vpc",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeVPCCreateTool_Success(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{
		ID:          999,
		Label:       "test-vpc",
		Region:      "us-east",
		Description: "Test VPC",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(vpc))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "test-vpc",
		"region":  "us-east",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "test-vpc")
	assert.Contains(t, textContent.Text, "999")
}

// =============================================================================
// VPC Update Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCUpdateTool(cfg)

	assert.Equal(t, "linode_vpc_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "vpc_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVPCUpdateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id": float64(123),
		"label":  "new-label",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCUpdateTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "new-label",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCUpdateTool_Success(t *testing.T) {
	t.Parallel()

	vpc := linode.VPC{
		ID:          123,
		Label:       "updated-vpc",
		Region:      "us-east",
		Description: "Updated VPC",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(vpc))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"label":   "updated-vpc",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// =============================================================================
// VPC Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCDeleteTool(cfg)

	assert.Equal(t, "linode_vpc_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")
}

func TestLinodeVPCDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"vpc_id": float64(123)})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCDeleteTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"confirm": true})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

// =============================================================================
// VPC Subnet Create Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCSubnetCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	assert.Equal(t, "linode_vpc_subnet_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "vpc_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "ipv4")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVPCSubnetCreateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id": float64(123),
		"label":  "web-subnet",
		"ipv4":   "10.0.0.0/24",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should require confirm=true")
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCSubnetCreateTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "web-subnet",
		"ipv4":    "10.0.0.0/24",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCSubnetCreateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"ipv4":    "10.0.0.0/24",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeVPCSubnetCreateTool_MissingIPv4(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"label":   "web-subnet",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "ipv4 is required")
}

func TestLinodeVPCSubnetCreateTool_Success(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{
		ID:    50,
		Label: "web-subnet",
		IPv4:  "10.0.0.0/24",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123/subnets", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(subnet))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"label":   "web-subnet",
		"ipv4":    "10.0.0.0/24",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-subnet")
	assert.Contains(t, textContent.Text, "50")
}

// =============================================================================
// VPC Subnet Update Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCSubnetUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	assert.Equal(t, "linode_vpc_subnet_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "vpc_id")
	assert.Contains(t, props, "subnet_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeVPCSubnetUpdateTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":    float64(123),
		"subnet_id": float64(10),
		"label":     "updated-subnet",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCSubnetUpdateTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"subnet_id": float64(10),
		"label":     "updated-subnet",
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCSubnetUpdateTool_MissingSubnetID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"label":   "updated-subnet",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "subnet_id is required")
}

func TestLinodeVPCSubnetUpdateTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":    float64(123),
		"subnet_id": float64(10),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeVPCSubnetUpdateTool_Success(t *testing.T) {
	t.Parallel()

	subnet := linode.VPCSubnet{
		ID:    10,
		Label: "updated-subnet",
		IPv4:  "10.0.0.0/24",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(subnet))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":    float64(123),
		"subnet_id": float64(10),
		"label":     "updated-subnet",
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// =============================================================================
// VPC Subnet Delete Tool Tests (Write)
// =============================================================================

func TestNewLinodeVPCSubnetDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	assert.Equal(t, "linode_vpc_subnet_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")
}

func TestLinodeVPCSubnetDeleteTool_MissingConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":    float64(123),
		"subnet_id": float64(10),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeVPCSubnetDeleteTool_MissingVPCID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"subnet_id": float64(10),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "vpc_id is required")
}

func TestLinodeVPCSubnetDeleteTool_MissingSubnetID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":  float64(123),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "subnet_id is required")
}

func TestLinodeVPCSubnetDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/vpcs/123/subnets/10", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"vpc_id":    float64(123),
		"subnet_id": float64(10),
		"confirm":   true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted")
}
