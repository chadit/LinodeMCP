package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewVersionTool creates a version info tool.
func NewVersionTool(_ *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"version",
		"Returns LinodeMCP server version and build information",
		toolschemas.Schema("linode.mcp.v1.VersionInput"),
	)

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		return MarshalProtoToolResponse(VersionResponseProto())
	}

	return tool, profiles.CapMeta, handler
}

// VersionResponseProto builds the canonical VersionResponse proto from the
// build-time metadata. The `version` MCP tool and the CLI `version` subcommand
// both serialize this message so the two surfaces (and the Python server) emit
// the same field set.
func VersionResponseProto() *linodev1.VersionResponse {
	info := appinfo.Get()

	return &linodev1.VersionResponse{
		Version:    info.Version,
		ApiVersion: info.APIVersion,
		BuildDate:  info.BuildDate,
		Commit:     info.Commit,
		Platform:   info.Platform,
	}
}
