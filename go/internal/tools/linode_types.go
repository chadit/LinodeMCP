package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeTypesListTool creates a tool for listing Linode instance types.
func NewLinodeTypesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_types_list",
		mcp.WithDescription("Lists all available Linode instance types (plans) with pricing information. Can filter by class (standard, dedicated, gpu, highmem, premium)."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("class",
			mcp.Description("Filter types by class (standard, dedicated, gpu, highmem, premium)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeTypesListRequest(ctx, &request, cfg)
	}

	return tool, handler
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
		types = filterByField(types, classFilter, func(t linode.InstanceType) string {
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

	return marshalToolResponse(response)
}
