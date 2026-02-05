//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeSSHKeyCreateTool creates a tool for creating an SSH key.
func NewLinodeSSHKeyCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_sshkey_create",
		mcp.WithDescription("Creates a new SSH key in your Linode profile. The key can then be used when deploying new Linode instances."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("A label for the SSH key (must be unique)"),
		),
		mcp.WithString("ssh_key",
			mcp.Required(),
			mcp.Description("The public SSH key in authorized_keys format (e.g., 'ssh-rsa AAAA...')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeyCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeSSHKeyCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	label := request.GetString("label", "")
	sshKey := request.GetString("ssh_key", "")

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if err := validateSSHKey(sshKey); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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
		SSHKey  *linode.SSHKey `json:"ssh_key"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message: fmt.Sprintf("SSH key '%s' created successfully", createdKey.Label),
		SSHKey:  createdKey,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeSSHKeyDeleteTool creates a tool for deleting an SSH key.
func NewLinodeSSHKeyDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_sshkey_delete",
		mcp.WithDescription("Deletes an SSH key from your Linode profile. This will not affect any instances already using this key."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("sshkey_id",
			mcp.Required(),
			mcp.Description("The ID of the SSH key to delete"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeyDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeSSHKeyDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	sshKeyID := request.GetInt("sshkey_id", 0)

	if sshKeyID == 0 {
		return mcp.NewToolResultError("sshkey_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteSSHKey(ctx, sshKeyID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete SSH key %d: %v", sshKeyID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		SSHKeyID int    `json:"sshkey_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:  fmt.Sprintf("SSH key %d deleted successfully", sshKeyID),
		SSHKeyID: sshKeyID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
