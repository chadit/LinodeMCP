package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	paramDatabaseEngineID       = "engine_id"
	paramDatabaseAllowList      = "allow_list"
	paramDatabaseClusterSize    = "cluster_size"
	paramDatabaseEngine         = "engine"
	paramDatabaseEngineConfig   = "engine_config"
	paramDatabaseFork           = "fork"
	paramDatabaseLabel          = "label"
	paramDatabasePrivateNetwork = "private_network"
	paramDatabaseRegion         = "region"
	paramDatabaseSSLConnection  = "ssl_connection"
	paramDatabaseType           = "type"

	databaseEnginesPageSizeMin = 25
	databaseEnginesPageSizeMax = 500

	databaseInstancesPageSizeMin = 25
	databaseInstancesPageSizeMax = 500
)

// NewLinodeDatabaseEngineListTool creates a tool for listing Managed Database engines.
func NewLinodeDatabaseEngineListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_engine_list",
		mcp.WithDescription("Lists available Managed Database engines with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseEnginesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseMySQLConfigGetTool creates a tool for listing MySQL Managed Database advanced parameters.
func NewLinodeDatabaseMySQLConfigGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_config_get",
		mcp.WithDescription("Lists MySQL Managed Database advanced parameters."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseMySQLConfigGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceListTool creates a tool for listing Managed Database instances.
func NewLinodeDatabaseInstanceListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_list",
		mcp.WithDescription("Lists Managed Database instances with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstancesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceCreateTool creates a tool for creating or restoring a MySQL Managed Database instance.
func NewLinodeDatabaseInstanceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_create",
		mcp.WithDescription("Creates or restores a MySQL Managed Database instance. This creates a billable resource."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString(paramDatabaseLabel, mcp.Required(), mcp.Description("Label for the database instance.")),
		mcp.WithString(paramDatabaseType, mcp.Required(), mcp.Description("Linode type for the database instance.")),
		mcp.WithString(paramDatabaseEngine, mcp.Required(), mcp.Description("Database engine ID, for example mysql/8.0.26.")),
		mcp.WithString(paramDatabaseRegion, mcp.Required(), mcp.Description("Region for the database instance.")),
		mcp.WithString(paramDatabaseAllowList, mcp.Description("JSON array of CIDR strings allowed to connect (optional).")),
		mcp.WithNumber(paramDatabaseClusterSize, mcp.Description("Number of nodes in the cluster (optional).")),
		mcp.WithString(paramDatabaseEngineConfig, mcp.Description("JSON object of MySQL engine configuration values (optional).")),
		mcp.WithString(paramDatabaseFork, mcp.Description("JSON object describing source database fork/restore settings (optional).")),
		mcp.WithBoolean(paramDatabasePrivateNetwork, mcp.Description("Whether to use private networking (optional).")),
		mcp.WithBoolean(paramDatabaseSSLConnection, mcp.Description("Whether to require SSL connections (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm database creation. This creates a billable resource.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseEngineGetTool creates a tool for getting one Managed Database engine.
func NewLinodeDatabaseEngineGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_engine_get",
		mcp.WithDescription("Retrieves a single Managed Database engine by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString(
			paramDatabaseEngineID,
			mcp.Description("The Managed Database engine ID to retrieve, for example mysql/8.0.26 (required)."),
			mcp.Required(),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseEngineGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleDatabaseEngineGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	engineID, validationMessage := databaseEngineIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	engine, err := client.GetDatabaseEngine(ctx, engineID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database engine: %v", err)), nil
	}

	return MarshalToolResponse(engine)
}

func handleDatabaseMySQLConfigGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	mysqlConfig, err := client.GetDatabaseMySQLConfig(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve MySQL Managed Database advanced parameters: %v", err)), nil
	}

	return MarshalToolResponse(mysqlConfig)
}

func handleDatabaseEnginesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := databaseEnginesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	engines, err := client.ListDatabaseEngines(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database engines: %v", err)), nil
	}

	return FormatListResponse(engines, nil, "database_engines")
}

func handleDatabaseInstancesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := databaseInstancesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instances, err := client.ListDatabaseInstances(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database instances: %v", err)), nil
	}

	return FormatListResponse(instances, nil, "database_instances")
}

func handleDatabaseInstanceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a billable Managed Database instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := databaseInstanceCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.CreateDatabaseInstance(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create Managed Database instance: %v", err)), nil
	}

	response := struct {
		Message  string                   `json:"message"`
		Instance *linode.DatabaseInstance `json:"database_instance"`
	}{
		Message:  fmt.Sprintf("Managed Database instance '%s' (ID: %d) created", instance.Label, instance.ID),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

func databaseEnginesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", databaseEnginesPageSizeMin, databaseEnginesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func databaseInstancesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", databaseInstancesPageSizeMin, databaseInstancesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func databaseEngineIDFromTool(request *mcp.CallToolRequest) (string, string) {
	engineID, ok := request.GetArguments()[paramDatabaseEngineID].(string)
	if !ok || strings.TrimSpace(engineID) == "" {
		return "", "engine_id must be a non-empty string"
	}

	if engineID != strings.TrimSpace(engineID) || strings.Contains(engineID, "?") || strings.Contains(engineID, "#") || strings.Contains(engineID, "..") {
		return "", "engine_id must not contain query, fragment, or traversal segments"
	}

	parts := strings.Split(engineID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "engine_id must use the engine/version format"
	}

	return engineID, ""
}

func databaseInstanceCreateRequestFromTool(request *mcp.CallToolRequest) (linode.CreateDatabaseInstanceRequest, string) {
	args := request.GetArguments()

	label, validationMessage := requiredStringArg(args, paramDatabaseLabel)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	databaseType, validationMessage := requiredStringArg(args, paramDatabaseType)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	engine, validationMessage := requiredStringArg(args, paramDatabaseEngine)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	region, validationMessage := requiredStringArg(args, paramDatabaseRegion)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	req := linode.CreateDatabaseInstanceRequest{Label: label, Type: databaseType, Engine: engine, Region: region}

	if allowListJSON := request.GetString(paramDatabaseAllowList, ""); allowListJSON != "" {
		var allowList []string
		if err := json.Unmarshal([]byte(allowListJSON), &allowList); err != nil {
			return linode.CreateDatabaseInstanceRequest{}, fmt.Sprintf("invalid allow_list JSON: %v", err)
		}

		req.AllowList = allowList
	}

	if clusterSizeValue, ok := args[paramDatabaseClusterSize]; ok {
		clusterSize, ok := numberArgToInt(clusterSizeValue)
		if !ok || clusterSize < 1 {
			return linode.CreateDatabaseInstanceRequest{}, "cluster_size must be a positive integer"
		}

		req.ClusterSize = clusterSize
	}

	if engineConfigJSON := request.GetString(paramDatabaseEngineConfig, ""); engineConfigJSON != "" {
		var engineConfig map[string]any
		if err := json.Unmarshal([]byte(engineConfigJSON), &engineConfig); err != nil {
			return linode.CreateDatabaseInstanceRequest{}, fmt.Sprintf("invalid engine_config JSON: %v", err)
		}

		req.EngineConfig = engineConfig
	}

	if forkJSON := request.GetString(paramDatabaseFork, ""); forkJSON != "" {
		var fork map[string]any
		if err := json.Unmarshal([]byte(forkJSON), &fork); err != nil {
			return linode.CreateDatabaseInstanceRequest{}, fmt.Sprintf("invalid fork JSON: %v", err)
		}

		req.Fork = fork
	}

	privateNetwork, validationMessage := optionalBoolArg(args, paramDatabasePrivateNetwork)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	if privateNetwork != nil {
		req.PrivateNetwork = privateNetwork
	}

	sslConnection, validationMessage := optionalBoolArg(args, paramDatabaseSSLConnection)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	if sslConnection != nil {
		req.SSLConnection = sslConnection
	}

	return req, ""
}

func requiredStringArg(args map[string]any, key string) (string, string) {
	value, ok := args[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", key + " must be a non-empty string"
	}

	return value, ""
}

func numberArgToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}

		return int(typed), true
	default:
		return 0, false
	}
}

func optionalBoolArg(args map[string]any, key string) (*bool, string) {
	value, valueOK := args[key]
	if !valueOK {
		return nil, ""
	}

	boolValue, ok := value.(bool)
	if !ok {
		return nil, key + " must be a boolean"
	}

	return &boolValue, ""
}
