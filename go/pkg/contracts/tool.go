// Package contracts defines core interfaces for LinodeMCP tool integration.
package contracts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// Tool represents an MCP tool that can be registered with the server.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, params map[string]any) (*mcp.CallToolResult, error)
}
