package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// ErrEnvironmentNotFound is returned when the requested environment is not in the config.
var ErrEnvironmentNotFound = errors.New("environment not found in configuration")

// ErrLinodeConfigIncomplete is returned when API URL or token is missing.
var ErrLinodeConfigIncomplete = errors.New("linode configuration is incomplete: check your API URL and token")

// NewLinodeInstancesTool creates a tool for listing Linode instances.
func NewLinodeInstancesTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_instances_list",
		mcp.WithDescription("Lists Linode instances with optional filtering by status"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("status",
			mcp.Description("Filter instances by status (running, stopped, etc.)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstancesRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeInstancesRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	statusFilter := request.GetString("status", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	instances, err := client.ListInstances(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instances: %v", err)), nil
	}

	if statusFilter != "" {
		instances = filterInstancesByStatus(instances, statusFilter)
	}

	return formatInstancesResponse(instances, statusFilter)
}

func selectEnvironment(cfg *config.Config, environment string) (*config.EnvironmentConfig, error) {
	if environment != "" {
		if env, exists := cfg.Environments[environment]; exists {
			return &env, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrEnvironmentNotFound, environment)
	}

	selectedEnv, err := cfg.SelectEnvironment("default")
	if err != nil {
		return nil, fmt.Errorf("failed to select default environment: %w", err)
	}
	return selectedEnv, nil
}

func validateLinodeConfig(env *config.EnvironmentConfig) error {
	if env.Linode.APIURL == "" || env.Linode.Token == "" {
		return ErrLinodeConfigIncomplete
	}
	return nil
}

func filterInstancesByStatus(instances []linode.Instance, statusFilter string) []linode.Instance {
	var filtered []linode.Instance
	statusFilter = strings.ToLower(statusFilter)
	for _, instance := range instances {
		if strings.ToLower(instance.Status) == statusFilter {
			filtered = append(filtered, instance)
		}
	}
	return filtered
}

func formatInstancesResponse(instances []linode.Instance, statusFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count     int               `json:"count"`
		Filter    string            `json:"filter,omitempty"`
		Instances []linode.Instance `json:"instances"`
	}{
		Count:     len(instances),
		Instances: instances,
	}

	if statusFilter != "" {
		response.Filter = "status=" + statusFilter
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
