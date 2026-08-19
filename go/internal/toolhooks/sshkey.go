package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeSshkeyUpdatePreview reads the key so the preview can name the label
// being replaced, which the request alone does not carry.
func LinodeSshkeyUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	sshKeyID := request.GetInt("ssh_key_id", 0)
	label := request.GetString("label", "")

	result, err := tools.RunDryRunPreviewDetailed(
		ctx, request, cfg, "linode_sshkey_update", method, path,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetSSHKey(ctx, sshKeyID))
		},
		func(ctx context.Context, _ *linode.Client, state any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_sshkey_update", func() tools.DryRunDetails {
				return sshKeyUpdateSideEffects(state, label)
			})
		},
	)

	return wrapPreview("linode_sshkey_update", result, err)
}

// sshKeyUpdateSideEffects is the Tier B walk for a key update, read against the
// key as it stands now.
func sshKeyUpdateSideEffects(state any, newLabel string) tools.DryRunDetails {
	var details tools.DryRunDetails

	var fromLabel string
	if key, isKey := state.(*linode.SSHKey); isKey && key != nil {
		fromLabel = key.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	return details
}
