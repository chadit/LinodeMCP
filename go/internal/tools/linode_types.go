package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	paramTypeID               = "type_id"
	errTypeIDRequired         = "type_id must be a non-empty string"
	errTypeIDNoPathSeparators = "type_id must not contain '/', '?', '#', or '..'"
)

// NewLinodeTypeListTool creates a tool for listing Linode instance types.
func NewLinodeTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_type_list",
		mcp.WithDescription("Lists all available Linode instance types (plans) with pricing information. Can filter by class (standard, dedicated, gpu, highmem, premium)."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"class",
			mcp.Description("Filter types by class (standard, dedicated, gpu, highmem, premium)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeTypesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeTypeGetTool creates a tool for getting one Linode instance type.
func NewLinodeTypeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_type_get",
		mcp.WithDescription("Gets one Linode instance type (plan) by type_id."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			paramTypeID,
			mcp.Required(),
			mcp.Description("Linode type ID, for example g6-standard-2."),
		),
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

	instanceType, err := client.GetType(ctx, typeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode type: %v", err)), nil
	}

	return MarshalToolResponse(instanceType)
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

func handleLinodeTypesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	classFilter := request.GetString("class", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	types, err := client.ListTypes(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode types: %v", err)), nil
	}

	if classFilter != "" {
		types = FilterByField(types, classFilter, func(t linode.InstanceType) string {
			return t.Class
		})
	}

	return formatTypesResponse(types, classFilter)
}

func formatTypesResponse(types []linode.InstanceType, classFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count  int                   `json:"count"`
		Filter string                `json:"filter,omitempty"`
		Types  []linode.InstanceType `json:"types"`
	}{
		Count: len(types),
		Types: types,
	}

	if classFilter != "" {
		response.Filter = "class=" + classFilter
	}

	return MarshalToolResponse(response)
}
