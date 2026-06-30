package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	paramTypeID               = "type_id"
	errTypeIDRequired         = "type_id must be a non-empty string"
	errTypeIDNoPathSeparators = "type_id must not contain '/', '?', '#', or '..'"
)

// NewLinodeTypeListTool creates a tool for listing Linode instance types.
func NewLinodeTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_type_list",
		"Lists all available Linode instance types (plans) with pricing information. Can filter by class (standard, dedicated, gpu, highmem, premium).",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.InstanceType, error) {
			return client.ListTypesProto(ctx)
		},
		[]listFilterParam[*linodev1.InstanceType]{
			fieldFilter("class", "Filter types by class (standard, dedicated, gpu, highmem, premium)",
				func(t *linodev1.InstanceType) string { return t.GetClass() }),
		},
		instanceTypeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func instanceTypeListResponse(items []*linodev1.InstanceType, count int32, filter *string) *linodev1.InstanceTypeListResponse {
	return &linodev1.InstanceTypeListResponse{Count: count, Filter: filter, Types: items}
}

// NewLinodeTypeGetTool creates a tool for getting one Linode instance type.
func NewLinodeTypeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_type_get",
		"Gets one Linode instance type (plan) by type_id.",
		toolschemas.Schema("linode.mcp.v1.InstanceTypeGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeTypeGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeTypeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	typeID, validationErr := validateTypeID(request.GetString(paramTypeID, ""))
	if validationErr != "" {
		return mcp.NewToolResultError(validationErr), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instanceType, err := client.GetTypeProto(ctx, typeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode type: %v", err)), nil
	}

	return MarshalProtoToolResponse(instanceType)
}

func validateTypeID(typeID string) (string, string) {
	if typeID == "" || strings.TrimSpace(typeID) == "" {
		return "", errTypeIDRequired
	}

	if typeID != strings.TrimSpace(typeID) || strings.Contains(typeID, "/") || strings.Contains(typeID, "?") || strings.Contains(typeID, "#") || strings.Contains(typeID, "..") {
		return "", errTypeIDNoPathSeparators
	}

	return typeID, ""
}
