package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeDomainZoneFileGetTool creates a tool for getting a domain zone file.
func NewLinodeDomainZoneFileGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_zone_file_get",
		"Gets the rendered zone file for a specific domain by its ID.",
		toolschemas.Schema("linode.mcp.v1.DomainZoneFileGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainZoneFileGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeDomainZoneFileGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)

	if domainID <= 0 {
		return mcp.NewToolResultError("domain_id must be a positive integer"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	zoneFile, err := client.GetDomainZoneFileProto(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve zone file for domain %d: %v", domainID, err)), nil
	}

	return MarshalProtoToolResponse(zoneFile)
}
