package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeProfileTool creates a tool for retrieving Linode profile info.
func NewLinodeProfileTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newSimpleGetTool(cfg, "linode_profile",
		"Retrieves Linode user account profile information",
		func(ctx context.Context, client *linode.RetryableClient) (any, error) {
			return client.GetProfile(ctx)
		},
	)
}
