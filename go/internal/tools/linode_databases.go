package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	paramDatabaseEngineID       = "engine_id"
	paramDatabaseInstanceID     = "instance_id"
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
	paramDatabaseUpdates        = "updates"
	paramDatabaseVersion        = "version"

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

// NewLinodeDatabaseInstanceGetTool creates a tool for getting one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_get",
		mcp.WithDescription("Retrieves a single MySQL Managed Database instance by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to retrieve.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceGetRequest(ctx, &request, cfg)
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

// NewLinodeDatabaseInstanceUpdateTool creates a tool for updating one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_update",
		mcp.WithDescription("Updates a MySQL Managed Database instance."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to update.")),
		mcp.WithString(paramDatabaseAllowList, mcp.Description("JSON array of CIDR strings allowed to connect (optional).")),
		mcp.WithString(paramDatabaseEngineConfig, mcp.Description("JSON object of MySQL engine configuration values (optional).")),
		mcp.WithString(paramDatabaseLabel, mcp.Description("New label for the database instance (optional).")),
		mcp.WithString(paramDatabasePrivateNetwork, mcp.Description("JSON object of private network settings (optional).")),
		mcp.WithString(paramDatabaseType, mcp.Description("New Linode type for the database instance (optional).")),
		mcp.WithString(paramDatabaseUpdates, mcp.Description("JSON object of maintenance update settings (optional).")),
		mcp.WithString(paramDatabaseVersion, mcp.Description("New MySQL version for the database instance (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm database update.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceDeleteTool creates a tool for deleting one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_delete",
		mcp.WithDescription("Deletes a MySQL Managed Database instance. WARNING: This is irreversible."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to delete.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm database deletion. This action is irreversible.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
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

func handleDatabaseInstanceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.GetDatabaseInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve MySQL Managed Database instance: %v", err)), nil
	}

	return MarshalToolResponse(instance)
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

func handleDatabaseInstanceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates a Managed Database instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req, validationMessage := databaseInstanceUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.UpdateDatabaseInstance(ctx, instanceID, req)
	if err != nil {
		return mcp.NewToolResultError(formatDatabaseInstanceUpdateError(err)), nil
	}

	response := struct {
		Message  string                   `json:"message"`
		Instance *linode.DatabaseInstance `json:"database_instance"`
	}{
		Message:  fmt.Sprintf("Managed Database instance '%s' (ID: %d) updated", instance.Label, instance.ID),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

func handleDatabaseInstanceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This deletes a Managed Database instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteDatabaseInstance(ctx, instanceID); err != nil {
		return mcp.NewToolResultError(formatDatabaseInstanceDeleteError(instanceID, err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
	}{
		Message:    "Managed Database instance " + strconv.Itoa(instanceID) + " deleted",
		InstanceID: instanceID,
	}

	return MarshalToolResponse(response)
}

func formatDatabaseInstanceDeleteError(instanceID int, err error) string {
	return "Failed to delete Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
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

func databaseInstanceIDFromTool(request *mcp.CallToolRequest) (int, string) {
	instanceIDValue, hasInstanceID := request.GetArguments()[paramDatabaseInstanceID]
	if !hasInstanceID {
		return 0, "instance_id must be a positive integer"
	}

	instanceID, ok := numberArgToInt(instanceIDValue)
	if !ok || instanceID < 1 {
		return 0, "instance_id must be a positive integer"
	}

	return instanceID, ""
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

func formatDatabaseInstanceUpdateError(err error) string {
	return "Failed to update Managed Database instance: " + err.Error()
}

func databaseInstanceUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateDatabaseInstanceRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateDatabaseInstanceRequest{}

	var changed bool

	allowList, hasAllowList, validationMessage := optionalStringSliceJSONField(args, paramDatabaseAllowList, "allow_list")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasAllowList {
		req.AllowList = &allowList
		changed = true
	}

	engineConfig, hasEngineConfig, validationMessage := optionalMapJSONField(args, paramDatabaseEngineConfig, "engine_config")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasEngineConfig {
		req.EngineConfig = engineConfig
		changed = true
	}

	label, hasLabel, validationMessage := optionalStringField(args, paramDatabaseLabel)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasLabel {
		req.Label = &label
		changed = true
	}

	privateNetwork, hasPrivateNetwork, validationMessage := optionalMapJSONField(args, paramDatabasePrivateNetwork, "private_network")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasPrivateNetwork {
		req.PrivateNetwork = privateNetwork
		changed = true
	}

	databaseType, hasDatabaseType, validationMessage := optionalStringField(args, paramDatabaseType)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasDatabaseType {
		req.Type = &databaseType
		changed = true
	}

	updates, hasUpdates, validationMessage := optionalMapJSONField(args, paramDatabaseUpdates, "updates")
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasUpdates {
		req.Updates = updates
		changed = true
	}

	version, hasVersion, validationMessage := optionalStringField(args, paramDatabaseVersion)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasVersion {
		req.Version = &version
		changed = true
	}

	if !changed {
		return nil, "at least one update field must be provided"
	}

	return req, ""
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

func optionalStringSliceJSONField(args map[string]any, key, label string) ([]string, bool, string) {
	jsonValue, hasValue, validationMessage := optionalStringField(args, key)
	if validationMessage != "" || !hasValue {
		return nil, hasValue, validationMessage
	}

	var values []string
	if err := json.Unmarshal([]byte(jsonValue), &values); err != nil {
		return nil, false, fmt.Sprintf("invalid %s JSON: %v", label, err)
	}

	if values == nil {
		return nil, false, label + " must be a JSON array"
	}

	return values, true, ""
}

func optionalMapJSONField(args map[string]any, key, label string) (map[string]any, bool, string) {
	jsonValue, hasValue, validationMessage := optionalStringField(args, key)
	if validationMessage != "" || !hasValue {
		return nil, hasValue, validationMessage
	}

	var values map[string]any
	if err := json.Unmarshal([]byte(jsonValue), &values); err != nil {
		return nil, false, fmt.Sprintf("invalid %s JSON: %v", label, err)
	}

	if values == nil {
		return nil, false, label + " must be a JSON object"
	}

	return values, true, ""
}

func optionalStringField(args map[string]any, key string) (string, bool, string) {
	value, exists := args[key]
	if !exists {
		return "", false, ""
	}

	stringValue, isString := value.(string)
	if !isString || strings.TrimSpace(stringValue) == "" {
		return "", false, key + " must be a non-empty string"
	}

	return stringValue, true, ""
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
