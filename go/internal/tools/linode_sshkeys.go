package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeSSHKeysListTool creates a tool for listing SSH keys.
func NewLinodeSSHKeysListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_sshkeys_list",
		mcp.WithDescription("Lists all SSH keys associated with your Linode profile. Can filter by label."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter SSH keys by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeSSHKeysListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeSSHKeysListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	labelContains := request.GetString("label_contains", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	keys, err := client.ListSSHKeys(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSH keys: %v", err)), nil
	}

	if labelContains != "" {
		keys = filterSSHKeysByLabel(keys, labelContains)
	}

	return formatSSHKeysResponse(keys, labelContains)
}

func filterSSHKeysByLabel(keys []linode.SSHKey, labelContains string) []linode.SSHKey {
	var filtered []linode.SSHKey

	labelContains = strings.ToLower(labelContains)

	for _, key := range keys {
		if strings.Contains(strings.ToLower(key.Label), labelContains) {
			filtered = append(filtered, key)
		}
	}

	return filtered
}

func formatSSHKeysResponse(keys []linode.SSHKey, labelContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count   int             `json:"count"`
		Filter  string          `json:"filter,omitempty"`
		SSHKeys []linode.SSHKey `json:"ssh_keys"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Count:   len(keys),
		SSHKeys: keys,
	}

	if labelContains != "" {
		response.Filter = "label_contains=" + labelContains
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
