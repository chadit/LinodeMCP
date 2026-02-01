package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/version"
)

// NewVersionTool creates a version info tool.
func NewVersionTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("version",
		mcp.WithDescription("Returns LinodeMCP server version and build information"),
	)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		versionInfo := version.Get()

		jsonResponse, err := json.MarshalIndent(versionInfo, "", "  ")
		if err != nil {
			//nolint:nilerr // fallback to string format
			return mcp.NewToolResultText(versionInfo.String()), nil
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}

	return tool, handler
}
