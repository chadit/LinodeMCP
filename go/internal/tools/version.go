package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewVersionTool creates a version info tool.
func NewVersionTool(_ *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"version",
		mcp.WithDescription("Returns LinodeMCP server version and build information"),
	)

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		versionInfo := appinfo.Get()

		jsonResponse, err := json.MarshalIndent(versionInfo, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal version info: %w", err)
		}

		return mcp.NewToolResultText(string(jsonResponse)), nil
	}

	return tool, profiles.CapMeta, handler
}
