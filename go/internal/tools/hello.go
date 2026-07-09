// Package tools provides built-in MCP tools for LinodeMCP.
package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewHelloTool creates a hello tool for smoke testing.
func NewHelloTool(_ *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"hello",
		"Responds with a friendly greeting from LinodeMCP",
		toolschemas.Schema("linode.mcp.v1.HelloInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := request.GetString("name", "World")

		return MarshalProtoToolResponse(&linodev1.HelloResponse{
			Message: fmt.Sprintf("Hello, %s! LinodeMCP server is running and ready.", name),
		})
	}

	return tool, profiles.CapMeta, handler
}
