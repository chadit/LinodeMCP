package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeSSHKeyGetTool creates a tool for getting a single SSH key by ID.
func NewLinodeSSHKeyGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_sshkey_get",
		mcp.WithDescription("Retrieves details of a single SSH key by its ID"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"ssh_key_id",
			mcp.Required(),
			mcp.Description("The ID of the SSH key to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeyGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeSSHKeyGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	sshKeyID := request.GetInt("ssh_key_id", 0)

	if sshKeyID <= 0 {
		return mcp.NewToolResultError("ssh_key_id must be a positive integer"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	sshKey, err := client.GetSSHKey(ctx, sshKeyID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSH key: %v", err)), nil
	}

	return MarshalToolResponse(sshKey)
}

// NewLinodeSSHKeyListTool creates a tool for listing SSH keys.
func NewLinodeSSHKeyListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_sshkey_list",
		mcp.WithDescription("Lists all SSH keys associated with your Linode profile. Can filter by label."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"label_contains",
			mcp.Description("Filter SSH keys by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeysListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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
		keys = FilterByContains(keys, labelContains, func(k linode.SSHKey) string {
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

	return MarshalToolResponse(response)
}
