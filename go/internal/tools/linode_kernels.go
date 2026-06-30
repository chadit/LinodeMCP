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
	kernelsPageSizeMin    = 25
	kernelsPageSizeMax    = 500
	kernelIDPrefixLinode  = "linode"
	errKernelIDIdentifier = "kernel_id must be a kernel identifier like linode/latest-64bit"
)

// NewLinodeKernelListTool creates a tool for listing Linode kernels.
func NewLinodeKernelListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginated(
		cfg,
		"linode_kernel_list",
		"Lists available Linode kernels with optional pagination.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.Kernel, error) {
			return client.ListKernelsProto(ctx, page, pageSize)
		},
		kernelsPaginationFromTool,
		nil,
		kernelListResponse,
	)

	return tool, profiles.CapRead, handler
}

func kernelListResponse(items []*linodev1.Kernel, count int32, filter *string) *linodev1.KernelListResponse {
	return &linodev1.KernelListResponse{Count: count, Filter: filter, Kernels: items}
}

// NewLinodeKernelGetTool creates a tool for retrieving one Linode kernel.
func NewLinodeKernelGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_kernel_get",
		"Gets one Linode kernel by ID.",
		toolschemas.Schema("linode.mcp.v1.KernelGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleKernelGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	kernel, err := client.GetKernelProto(ctx, kernelID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve kernel: %v", err)), nil
	}

	return MarshalProtoToolResponse(kernel)
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
