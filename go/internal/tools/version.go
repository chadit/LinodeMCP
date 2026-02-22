package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/appinfo"
)

// NewVersionTool creates a version info tool.
func NewVersionTool() (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("version",
		mcp.WithDescription("Returns LinodeMCP server version and build information"),
	)

	handler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		versionInfo := appinfo.Get()

		jsonResponse, err := json.MarshalIndent(versionInfo, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal version info: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}

	return tool, handler
}
