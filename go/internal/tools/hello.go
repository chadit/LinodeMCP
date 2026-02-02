// Package tools provides built-in MCP tools for LinodeMCP.
package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewHelloTool creates a hello tool for smoke testing.
func NewHelloTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("hello",
		mcp.WithDescription("Responds with a friendly greeting from LinodeMCP"),
		mcp.WithString("name",
			mcp.Description("Name to include in the greeting (optional)"),
		),
	)

	handler := func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "World")
		message := fmt.Sprintf("Hello, %s! LinodeMCP server is running and ready.", name)

		return mcp.NewToolResultText(message), nil
	}

	return tool, handler
}
