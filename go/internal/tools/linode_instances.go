package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// boolTrue is used for boolean string comparison in filter functions.
const boolTrue = "true"

// ErrEnvironmentNotFound is returned when the requested environment is not in the config.
var ErrEnvironmentNotFound = errors.New("environment not found in configuration")

// ErrLinodeConfigIncomplete is returned when API URL or token is missing.
var ErrLinodeConfigIncomplete = errors.New("linode configuration is incomplete: check your API URL and token")

// ErrInstanceIDRequired is returned when an instance ID is required but not provided.
var ErrInstanceIDRequired = errors.New("instance_id is required")

// ErrInvalidInstanceID is returned when the instance ID is not a valid integer.
var ErrInvalidInstanceID = errors.New("instance_id must be a valid integer")

// NewLinodeInstanceGetTool creates a tool for getting a single Linode instance by ID.
func NewLinodeInstanceGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_instance_get",
		mcp.WithDescription("Retrieves details of a single Linode instance by its ID"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("instance_id",
			mcp.Description("The ID of the Linode instance to retrieve (required)"),
			mcp.Required(),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeInstanceGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	instanceIDStr := request.GetString("instance_id", "")

	if instanceIDStr == "" {
		return mcp.NewToolResultError(ErrInstanceIDRequired.Error()), nil
	}

	instanceID, err := strconv.Atoi(instanceIDStr)
	if err != nil {
		return mcp.NewToolResultError(ErrInvalidInstanceID.Error()), nil //nolint:nilerr // MCP tool errors are returned as tool results
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance: %v", err)), nil
	}

	jsonResponse, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

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
