package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeSSHKeyCreateTool creates a tool for creating an SSH key.
func NewLinodeSSHKeyCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_sshkey_create",
		"Creates a new SSH key in your Linode profile. The key can then be used when deploying new Linode instances. Pass dry_run=true to preview without creating.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(), mcp.Description("A label for the SSH key (must be unique)")),
			mcp.WithString("ssh_key", mcp.Required(),
				mcp.Description("The public SSH key in authorized_keys format (e.g., 'ssh-rsa AAAA...')")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm SSH key creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeSSHKeyCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeSSHKeyCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	sshKey := request.GetString("ssh_key", "")

	if IsDryRun(request) {
		if label == "" {
			return mcp.NewToolResultError("label is required"), nil
		}

		if err := validateSSHKey(sshKey); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_sshkey_create", httpMethodPost, "/profile/sshkeys", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return sshKeyCreateSideEffects(ctx, label)
			})
	}

	if result := RequireConfirm(request, "This adds an SSH key to your Linode profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if err := validateSSHKey(sshKey); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateSSHKeyRequest{
		Label:  label,
		SSHKey: sshKey,
	}

	createdKey, err := client.CreateSSHKey(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create SSH key: %v", err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		SSHKey  *linode.SSHKey `json:"ssh_key"`
	}{
		Message: fmt.Sprintf("SSH key '%s' created successfully", createdKey.Label),
		SSHKey:  createdKey,
	}

	return MarshalToolResponse(response)
}

// NewLinodeSSHKeyUpdateTool creates a tool for updating an SSH key.
func NewLinodeSSHKeyUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_sshkey_update",
		"Updates the label for an SSH key in your Linode profile. Pass dry_run=true to preview without updating.",
		[]mcp.ToolOption{
			mcp.WithNumber("sshkey_id", mcp.Required(), mcp.Description("The ID of the SSH key to update")),
			mcp.WithString("label", mcp.Required(), mcp.Description("The new label for the SSH key")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm SSH key update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeSSHKeyUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeSSHKeyUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	sshKeyID := request.GetInt("sshkey_id", 0)
	label := request.GetString("label", "")

	if IsDryRun(request) {
		if sshKeyID <= 0 {
			return mcp.NewToolResultError("sshkey_id must be a positive integer"), nil
		}

		if label == "" {
			return mcp.NewToolResultError("label is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_sshkey_update", "PUT",
			fmt.Sprintf("/profile/sshkeys/%d", sshKeyID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetSSHKey(ctx, sshKeyID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return sshKeyUpdateSideEffects(ctx, state, label)
			})
	}

	if result := RequireConfirm(request, "This updates an SSH key in your Linode profile. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if sshKeyID <= 0 {
		return mcp.NewToolResultError("sshkey_id must be a positive integer"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateSSHKeyRequest{Label: label}

	updatedKey, err := client.UpdateSSHKey(ctx, sshKeyID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("SSH key %d failed to change label: %v", sshKeyID, err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		SSHKey  *linode.SSHKey `json:"ssh_key"`
	}{
		Message: fmt.Sprintf("SSH key %d updated successfully", sshKeyID),
		SSHKey:  updatedKey,
	}

	return MarshalToolResponse(response)
}

// NewLinodeSSHKeyDeleteTool creates a tool for deleting an SSH key.
func NewLinodeSSHKeyDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_sshkey_delete",
		"Deletes an SSH key from your Linode profile. This will not affect any instances already using this key. Pass dry_run=true to preview without deleting.",
		[]mcp.ToolOption{
			mcp.WithNumber("sshkey_id", mcp.Required(), mcp.Description("The ID of the SSH key to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm SSH key deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeSSHKeyDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeSSHKeyDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_sshkey_delete",
		IDParam:        "sshkey_id",
		Method:         httpMethodDelete,
		PathPattern:    "/profile/sshkeys/%d",
		ConfirmMessage: "This removes an SSH key from your Linode profile. Set confirm=true to proceed.",
		SuccessFormat:  "SSH key %d removed successfully",
		FetchState:     func(ctx context.Context, c *linode.Client, id int) (any, error) { return c.GetSSHKey(ctx, id) },
		Execute:        func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteSSHKey(ctx, id) },
	})
}
