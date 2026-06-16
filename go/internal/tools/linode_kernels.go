package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	kernelsPageSizeMin    = 25
	kernelsPageSizeMax    = 500
	kernelIDPrefixLinode  = "linode"
	errKernelIDIdentifier = "kernel_id must be a kernel identifier like linode/latest-64bit"
)

// NewLinodeKernelListTool creates a tool for listing Linode kernels.
func NewLinodeKernelListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_kernel_list",
		mcp.WithDescription("Lists available Linode kernels with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeKernelsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeKernelGetTool creates a tool for retrieving one Linode kernel.
func NewLinodeKernelGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_kernel_get",
		mcp.WithDescription("Gets one Linode kernel by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString("kernel_id", mcp.Required(), mcp.Description("Kernel ID, such as linode/latest-64bit.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleKernelGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeKernelsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := kernelsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	kernels, err := client.ListKernels(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode kernels: %v", err)), nil
	}

	return FormatListResponse(kernels, nil, "kernels")
}

func handleKernelGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	kernelID, ok := getKernelIDArg(request)
	if !ok {
		return mcp.NewToolResultError(errKernelIDIdentifier), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	kernel, err := client.GetKernel(ctx, kernelID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve kernel: %v", err)), nil
	}

	return MarshalToolResponse(kernel)
}

func kernelsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", kernelsPageSizeMin, kernelsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func getKernelIDArg(request *mcp.CallToolRequest) (string, bool) {
	raw, ok := request.GetArguments()["kernel_id"].(string)
	if !ok {
		return "", false
	}

	kernelID := strings.TrimSpace(raw)
	if kernelID == "" || strings.ContainsAny(kernelID, "?#") || strings.Contains(kernelID, "..") {
		return "", false
	}

	prefix, name, found := strings.Cut(kernelID, "/")
	if !found || prefix != kernelIDPrefixLinode || name == "" || strings.Contains(name, "/") {
		return "", false
	}

	return kernelID, true
}
