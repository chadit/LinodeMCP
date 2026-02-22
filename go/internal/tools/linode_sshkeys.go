package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeSSHKeysListTool creates a tool for listing SSH keys.
func NewLinodeSSHKeysListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_sshkeys_list",
		mcp.WithDescription("Lists all SSH keys associated with your Linode profile. Can filter by label."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter SSH keys by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeysListRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeSSHKeysListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	labelContains := request.GetString("label_contains", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	keys, err := client.ListSSHKeys(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSH keys: %v", err)), nil
	}

	if labelContains != "" {
		keys = filterByContains(keys, labelContains, func(k linode.SSHKey) string {
			return k.Label
		})
	}

	response := struct {
		Count   int             `json:"count"`
		Filter  string          `json:"filter,omitempty"`
		SSHKeys []linode.SSHKey `json:"ssh_keys"`
	}{
		Count:   len(keys),
		SSHKeys: keys,
	}

	if labelContains != "" {
		response.Filter = "label_contains=" + labelContains
	}

	return marshalToolResponse(response)
}
