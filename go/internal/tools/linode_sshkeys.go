package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeSSHKeyGetTool creates a tool for getting a single SSH key by ID.
func NewLinodeSSHKeyGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_sshkey_get",
		"Retrieves details of a single SSH key by its ID",
		toolschemas.Schema("linode.mcp.v1.SSHKeyGetInput"),
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

	sshKey, err := client.GetSSHKeyProto(ctx, sshKeyID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSH key: %v", err)), nil
	}

	return MarshalProtoToolResponse(sshKey)
}

// NewLinodeSSHKeyListTool creates a tool for listing SSH keys.
func NewLinodeSSHKeyListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListTool(
		cfg,
		"linode_sshkey_list",
		"Lists all SSH keys associated with your Linode profile. Can filter by label.",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.SSHKey, error) {
			return client.ListSSHKeysProto(ctx)
		},
		[]listFilterParam[*linodev1.SSHKey]{
			containsFilter("label_contains", "Filter SSH keys by label containing this string (case-insensitive)",
				func(k *linodev1.SSHKey) string { return k.GetLabel() }),
		},
		sshKeyListResponse,
	)

	return tool, profiles.CapRead, handler
}

func sshKeyListResponse(items []*linodev1.SSHKey, count int32, filter *string) *linodev1.SSHKeyListResponse {
	return &linodev1.SSHKeyListResponse{Count: count, Filter: filter, SshKeys: items}
}
