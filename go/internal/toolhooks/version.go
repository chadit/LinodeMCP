package toolhooks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// VersionAnswer reports the build metadata the binary carries.
//
// It serializes the same message the CLI's version verb does, so the two
// surfaces cannot drift a field apart.
func VersionAnswer(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ *config.Config,
) (*mcp.CallToolResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("version canceled: %w", err)
	}

	return answeredBy(tools.MarshalProtoToolResponse(tools.VersionResponseProto()))
}
