package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeAccountTool creates a tool for retrieving Linode account information.
func NewLinodeAccountTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newSimpleGetTool(cfg, "linode_account",
		"Retrieves the authenticated user's Linode account information including billing details and capabilities",
		func(ctx context.Context, client *linode.RetryableClient) (any, error) {
			return client.GetAccount(ctx)
		},
	)
}
