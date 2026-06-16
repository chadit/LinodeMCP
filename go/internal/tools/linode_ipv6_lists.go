package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	ipv6ListPageSizeMin = 25
	ipv6ListPageSizeMax = 500
)

type ipv6ListFunc[T any] func(context.Context, *linode.Client, int, int) (*linode.PaginatedResponse[T], string)

func newLinodeIPv6ListTool[T any](
	cfg *config.Config,
	name string,
	description string,
	listFn ipv6ListFunc[T],
) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		name,
		description,
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
			return handleLinodeIPv6ListRequest(ctx, request, cfg, name, listFn)
		},
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeIPv6ListRequest[T any](
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName string,
	listFn ipv6ListFunc[T],
) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := ipv6ListPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	items, failureMessage := listFn(ctx, client, page, pageSize)
	if failureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve " + toolName + ": " + failureMessage), nil
	}

	return MarshalToolResponse(items)
}

func ipv6ListPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", ipv6ListPageSizeMin, ipv6ListPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}
