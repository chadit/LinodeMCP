package tools

import (
	"context"
	"errors"
	"fmt"
	"strconv"

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
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("instance_id",
			mcp.Description("The ID of the Linode instance to retrieve (required)"),
			mcp.Required(),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstanceGetRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeInstanceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, err := parseInstanceID(request.GetString("instance_id", ""))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instance: %v", err)), nil
	}

	return marshalToolResponse(instance)
}

// NewLinodeInstancesTool creates a tool for listing Linode instances.
func NewLinodeInstancesTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_instances_list",
		mcp.WithDescription("Lists Linode instances with optional filtering by status"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("status",
			mcp.Description("Filter instances by status (running, stopped, etc.)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeInstancesRequest(ctx, &request, cfg)
	}

	return tool, handler
}

func handleLinodeInstancesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	statusFilter := request.GetString("status", "")

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instances, err := client.ListInstances(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode instances: %v", err)), nil
	}

	if statusFilter != "" {
		instances = filterByField(instances, statusFilter, func(inst linode.Instance) string {
			return inst.Status
		})
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

// parseInstanceID validates and converts the instance ID string to an integer.
func parseInstanceID(raw string) (int, error) {
	if raw == "" {
		return 0, ErrInstanceIDRequired
	}

	instanceID, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidInstanceID, raw)
	}

	return instanceID, nil
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

	return marshalToolResponse(response)
}
