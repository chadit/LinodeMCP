package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
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
	paramDatabaseTypeID         = "type_id"
	paramDatabaseUpdates        = "updates"
	paramDatabaseVersion        = "version"

	databaseEnginesPageSizeMin = 25
	databaseEnginesPageSizeMax = 500

	databaseTypesPageSizeMin = 25
	databaseTypesPageSizeMax = 500

	databaseInstancesPageSizeMin = 25
	databaseInstancesPageSizeMax = 500

	dbMySQLInstancesPath      = "/databases/mysql/instances"
	dbPostgreSQLInstancesPath = "/databases/postgresql/instances"

	dbMessagePrefixMySQL      = "Managed Database"
	dbMessagePrefixPostgreSQL = "PostgreSQL Managed Database"

	responseKeyDatabaseInstances = "database_instances"
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

// NewLinodeDatabaseTypeListTool creates a tool for listing Managed Database node types.
func NewLinodeDatabaseTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_type_list",
		mcp.WithDescription("Lists available Managed Database node types with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseTypesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseTypeGetTool creates a tool for getting one Managed Database node type.
func NewLinodeDatabaseTypeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_type_get",
		mcp.WithDescription("Retrieves a single Managed Database node type by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString(
			paramDatabaseTypeID,
			mcp.Description("The Managed Database type ID to retrieve, for example g6-dedicated-1 (required)."),
			mcp.Required(),
		),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseTypeGetRequest(ctx, &request, cfg)
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

// NewLinodeDatabasePostgreSQLConfigGetTool creates a tool for listing PostgreSQL Managed Database advanced parameters.
func NewLinodeDatabasePostgreSQLConfigGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_config_get",
		mcp.WithDescription("Lists PostgreSQL Managed Database advanced parameters."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLConfigGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceListTool creates a tool for listing Managed Database instances.
func NewLinodeDatabaseInstanceListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_list",
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

// NewLinodeDatabaseAllInstancesListTool creates a tool for listing Managed
// Database instances across every engine. Unlike the MySQL and PostgreSQL
// list tools, this one hits the cross-engine /databases/instances endpoint.
func NewLinodeDatabaseAllInstancesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_instance_list",
		mcp.WithDescription("Lists Managed Database instances across all engines with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseAllInstancesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabasePostgreSQLInstanceListTool creates a tool for listing PostgreSQL Managed Database instances.
func NewLinodeDatabasePostgreSQLInstanceListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_list",
		mcp.WithDescription("Lists PostgreSQL Managed Database instances with optional pagination."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
		mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstancesListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceGetTool creates a tool for getting one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_get",
		mcp.WithDescription("Retrieves a single MySQL Managed Database instance by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to retrieve.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabasePostgreSQLInstanceGetTool creates a tool for getting one PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_get",
		mcp.WithDescription("Retrieves a single PostgreSQL Managed Database instance by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID to retrieve.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceSSLGetTool creates a tool for getting a MySQL Managed Database SSL CA certificate.
func NewLinodeDatabaseInstanceSSLGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_ssl_get",
		mcp.WithDescription("Retrieves the SSL CA certificate for a MySQL Managed Database instance by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID whose SSL CA certificate to retrieve.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceSSLGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabasePostgreSQLInstanceSSLGetTool creates a tool for getting a PostgreSQL Managed Database SSL CA certificate.
func NewLinodeDatabasePostgreSQLInstanceSSLGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_ssl_get",
		mcp.WithDescription("Retrieves the SSL CA certificate for a PostgreSQL Managed Database instance by ID."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID whose SSL CA certificate to retrieve.")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceSSLGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeDatabaseInstanceCredentialsGetTool creates a tool for getting MySQL Managed Database credentials.
func NewLinodeDatabaseInstanceCredentialsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_credentials_get",
		mcp.WithDescription("Retrieves credentials for a MySQL Managed Database instance by ID. Pass dry_run=true to preview the request without retrieving the secret."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID whose credentials to retrieve.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm retrieving the database credentials. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceCredentialsGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool creates a tool for getting PostgreSQL Managed Database credentials.
func NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_credentials_get",
		mcp.WithDescription("Retrieves credentials for a PostgreSQL Managed Database instance by ID. Pass dry_run=true to preview the request without retrieving the secret."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID whose credentials to retrieve.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm retrieving the database credentials. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceCredentialsGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceCredentialsResetTool creates a tool for resetting MySQL Managed Database credentials.
func NewLinodeDatabaseInstanceCredentialsResetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_credentials_reset",
		mcp.WithDescription("Resets credentials for a MySQL Managed Database instance by ID. Pass dry_run=true to preview without resetting."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID whose credentials to reset.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm resetting database credentials. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceCredentialsResetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool creates a tool for resetting PostgreSQL Managed Database credentials.
func NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_credentials_reset",
		mcp.WithDescription("Resets credentials for a PostgreSQL Managed Database instance by ID. Pass dry_run=true to preview without resetting."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID whose credentials to reset.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm resetting PostgreSQL database credentials. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceCredentialsResetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceCreateTool creates a tool for creating or restoring a MySQL Managed Database instance.
func NewLinodeDatabaseInstanceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceCreateTool(
		cfg,
		"linode_database_mysql_instance_create",
		"Creates or restores a MySQL Managed Database instance. This creates a billable resource.",
		"Database engine ID, for example mysql/8.0.26.",
		"JSON object of MySQL engine configuration values (optional).",
		handleDatabaseInstanceCreateRequest,
	)
}

// NewLinodeDatabasePostgreSQLInstanceCreateTool creates a tool for creating or restoring a PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceCreateTool(
		cfg,
		"linode_database_postgresql_instance_create",
		"Creates or restores a PostgreSQL Managed Database instance. This creates a billable resource.",
		"PostgreSQL database engine ID, for example postgresql/16.",
		"JSON object of PostgreSQL engine configuration values (optional).",
		handleDatabasePostgreSQLInstanceCreateRequest,
	)
}

func newDatabaseInstanceCreateTool(
	cfg *config.Config,
	name string,
	description string,
	engineDescription string,
	engineConfigDescription string,
	handle func(context.Context, *mcp.CallToolRequest, *config.Config) (*mcp.CallToolResult, error),
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		name,
		mcp.WithDescription(description+" Pass dry_run=true to preview without creating."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithString(paramDatabaseLabel, mcp.Required(), mcp.Description("Label for the database instance.")),
		mcp.WithString(paramDatabaseType, mcp.Required(), mcp.Description("Linode type for the database instance.")),
		mcp.WithString(paramDatabaseEngine, mcp.Required(), mcp.Description(engineDescription)),
		mcp.WithString(paramDatabaseRegion, mcp.Required(), mcp.Description("Region for the database instance.")),
		mcp.WithArray(paramDatabaseAllowList, mcp.Description("CIDR strings allowed to connect (optional).")),
		mcp.WithNumber(paramDatabaseClusterSize, mcp.Description("Number of nodes in the cluster (optional).")),
		mcp.WithObject(paramDatabaseEngineConfig, mcp.Description(engineConfigDescription)),
		mcp.WithObject(paramDatabaseFork, mcp.Description("Object describing source database fork/restore settings (optional).")),
		mcp.WithBoolean(paramDatabasePrivateNetwork, mcp.Description("Whether to use private networking (optional).")),
		mcp.WithBoolean(paramDatabaseSSLConnection, mcp.Description("Whether to require SSL connections (optional).")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm database creation. This creates a billable resource. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handle(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceUpdateTool creates a tool for updating one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceUpdateTool(
		cfg,
		"linode_database_mysql_instance_update",
		"Updates a MySQL Managed Database instance.",
		"The MySQL Managed Database instance ID to update.",
		"JSON object of MySQL engine configuration values (optional).",
		"New MySQL version for the database instance (optional).",
		"This updates a Managed Database instance. Set confirm=true to proceed.",
		handleDatabaseInstanceUpdateRequest,
	)
}

// NewLinodeDatabasePostgreSQLInstanceUpdateTool creates a tool for updating one PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceUpdateTool(
		cfg,
		"linode_database_postgresql_instance_update",
		"Updates a PostgreSQL Managed Database instance.",
		"The PostgreSQL Managed Database instance ID to update.",
		"JSON object of PostgreSQL engine configuration values (optional).",
		"New PostgreSQL version for the database instance (optional).",
		"This updates a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		handleDatabasePostgreSQLInstanceUpdateRequest,
	)
}

func newDatabaseInstanceUpdateTool(
	cfg *config.Config,
	name string,
	description string,
	instanceIDDescription string,
	engineConfigDescription string,
	versionDescription string,
	confirmDescription string,
	handle func(context.Context, *mcp.CallToolRequest, *config.Config) (*mcp.CallToolResult, error),
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		name,
		mcp.WithDescription(description+" Pass dry_run=true to preview without modifying."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description(instanceIDDescription)),
		mcp.WithArray(paramDatabaseAllowList, mcp.Description("CIDR strings allowed to connect (optional).")),
		mcp.WithObject(paramDatabaseEngineConfig, mcp.Description(engineConfigDescription)),
		mcp.WithString(paramDatabaseLabel, mcp.Description("New label for the database instance (optional).")),
		mcp.WithString(paramDatabasePrivateNetwork, mcp.Description("JSON object of private network settings (optional).")),
		mcp.WithString(paramDatabaseType, mcp.Description("New Linode type for the database instance (optional).")),
		mcp.WithObject(paramDatabaseUpdates, mcp.Description("Object of maintenance update settings (optional).")),
		mcp.WithString(paramDatabaseVersion, mcp.Description(versionDescription)),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description(confirmDescription)),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handle(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceDeleteTool creates a tool for deleting one MySQL Managed Database instance.
func NewLinodeDatabaseInstanceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceDeleteTool(
		cfg,
		"linode_database_mysql_instance_delete",
		"Deletes a MySQL Managed Database instance. WARNING: This is irreversible. Pass dry_run=true to preview without deleting.",
		"The MySQL Managed Database instance ID to delete.",
		"Must be true to confirm database deletion. This action is irreversible. Ignored when dry_run=true.",
		handleDatabaseInstanceDeleteRequest,
	)
}

// NewLinodeDatabasePostgreSQLInstanceDeleteTool creates a tool for deleting one PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newDatabaseInstanceDeleteTool(
		cfg,
		"linode_database_postgresql_instance_delete",
		"Deletes a PostgreSQL Managed Database instance. WARNING: This is irreversible. Pass dry_run=true to preview without deleting.",
		"The PostgreSQL Managed Database instance ID to delete.",
		"Must be true to confirm PostgreSQL database deletion. This action is irreversible. Ignored when dry_run=true.",
		handleDatabasePostgreSQLInstanceDeleteRequest,
	)
}

// newDatabaseInstanceDeleteTool builds the two-stage-capable delete tool shared
// by the MySQL and PostgreSQL database delete handlers. Routing both through one
// builder keeps the structurally identical constructors below dupl's threshold.
func newDatabaseInstanceDeleteTool(
	cfg *config.Config,
	name string,
	description string,
	instanceIDDescription string,
	confirmDescription string,
	handle func(context.Context, *mcp.CallToolRequest, *config.Config) (*mcp.CallToolResult, error),
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		name,
		mcp.WithDescription(description+twoStageNote),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description(instanceIDDescription)),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description(confirmDescription)),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
		mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handle(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// NewLinodeDatabaseInstancePatchTool creates a tool for patching one MySQL Managed Database instance.
func NewLinodeDatabaseInstancePatchTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_patch",
		mcp.WithDescription("Applies security patches and updates to a MySQL Managed Database instance. Pass dry_run=true to preview without patching."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to patch.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm database patching. This may cause maintenance downtime for single-node clusters. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstancePatchRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabasePostgreSQLInstancePatchTool creates a tool for patching one PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstancePatchTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_patch",
		mcp.WithDescription("Applies security patches and updates to a PostgreSQL Managed Database instance. Pass dry_run=true to preview without patching."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID to patch.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm PostgreSQL database patching. This may cause maintenance downtime for single-node clusters. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstancePatchRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceSuspendTool creates a tool for suspending one active MySQL Managed Database instance.
func NewLinodeDatabaseInstanceSuspendTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_suspend",
		mcp.WithDescription("Suspends an active MySQL Managed Database instance. Pass dry_run=true to preview without suspending."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to suspend.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm suspending the database instance. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceSuspendRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabasePostgreSQLInstanceSuspendTool creates a tool for suspending one active PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceSuspendTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_suspend",
		mcp.WithDescription("Suspends an active PostgreSQL Managed Database instance. Pass dry_run=true to preview without suspending."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID to suspend.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm suspending the PostgreSQL database instance. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceSuspendRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabaseInstanceResumeTool creates a tool for resuming one suspended MySQL Managed Database instance.
func NewLinodeDatabaseInstanceResumeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_mysql_instance_resume",
		mcp.WithDescription("Resumes a suspended MySQL Managed Database instance. Pass dry_run=true to preview without resuming."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The MySQL Managed Database instance ID to resume.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm resuming the database instance. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabaseInstanceResumeRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// NewLinodeDatabasePostgreSQLInstanceResumeTool creates a tool for resuming one suspended PostgreSQL Managed Database instance.
func NewLinodeDatabasePostgreSQLInstanceResumeTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_database_postgresql_instance_resume",
		mcp.WithDescription("Resumes a suspended PostgreSQL Managed Database instance. Pass dry_run=true to preview without resuming."),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
		mcp.WithNumber(paramDatabaseInstanceID, mcp.Required(), mcp.Description("The PostgreSQL Managed Database instance ID to resume.")),
		mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm resuming the PostgreSQL database instance. Ignored when dry_run=true.")),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDatabasePostgreSQLInstanceResumeRequest(ctx, &request, cfg)
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

func handleDatabasePostgreSQLConfigGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	postgresqlConfig, err := client.GetDatabasePostgreSQLConfig(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve PostgreSQL Managed Database advanced parameters: %v", err)), nil
	}

	return MarshalToolResponse(postgresqlConfig)
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

func handleDatabasePostgreSQLInstanceGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := client.GetDatabasePostgreSQLInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve PostgreSQL Managed Database instance: %v", err)), nil
	}

	return MarshalToolResponse(instance)
}

func handleDatabaseInstanceSSLGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ssl, err := client.GetDatabaseInstanceSSL(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve MySQL Managed Database SSL certificate: %v", err)), nil
	}

	return MarshalToolResponse(ssl)
}

func handleDatabasePostgreSQLInstanceSSLGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ssl, err := client.GetDatabasePostgreSQLInstanceSSL(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve PostgreSQL Managed Database SSL certificate: %v", err)), nil
	}

	return MarshalToolResponse(ssl)
}

func handleDatabaseCredentialsGet(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, instancesPath, engineLabel string,
	fetchInstance func(ctx context.Context, c *linode.Client, id int) (any, error),
	fetchCredentials func(ctx context.Context, c *linode.Client, id int) (any, error),
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return runDatabaseInstanceActionDryRun(ctx, request, cfg, toolName, "GET", instancesPath, "credentials", fetchInstance)
	}

	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if result := RequireConfirm(request, "This retrieves Managed Database credentials. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	credentials, err := fetchCredentials(ctx, client, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve %s Managed Database credentials: %v", engineLabel, err)), nil
	}

	return MarshalToolResponse(credentials)
}

func handleDatabaseInstanceCredentialsGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleDatabaseCredentialsGet(
		ctx, request, cfg,
		"linode_database_mysql_instance_credentials_get", dbMySQLInstancesPath, "MySQL",
		func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstance(ctx, id)
		},
		func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstanceCredentials(ctx, id)
		},
	)
}

func handleDatabasePostgreSQLInstanceCredentialsGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleDatabaseCredentialsGet(
		ctx, request, cfg,
		"linode_database_postgresql_instance_credentials_get", dbPostgreSQLInstancesPath, "PostgreSQL",
		func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstance(ctx, id)
		},
		func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstanceCredentials(ctx, id)
		},
	)
}

func handleDatabaseInstanceCredentialsResetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return runDatabaseInstanceActionDryRun(ctx, request, cfg, "linode_database_mysql_instance_credentials_reset", httpMethodPost, dbMySQLInstancesPath, "credentials/reset",
			func(ctx context.Context, c *linode.Client, id int) (any, error) {
				return c.GetDatabaseInstance(ctx, id)
			})
	}

	if result := RequireConfirm(request, "This resets Managed Database credentials. Set confirm=true to proceed."); result != nil {
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

	credentials, err := client.ResetDatabaseInstanceCredentials(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to reset MySQL Managed Database credentials: %v", err)), nil
	}

	response := struct {
		Message     string                      `json:"message"`
		InstanceID  int                         `json:"instance_id"`
		Credentials *linode.DatabaseCredentials `json:"credentials"`
	}{
		Message:     "MySQL Managed Database credentials reset",
		InstanceID:  instanceID,
		Credentials: credentials,
	}

	return MarshalToolResponse(response)
}

func handleDatabasePostgreSQLInstanceCredentialsResetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return runDatabaseInstanceActionDryRun(ctx, request, cfg, "linode_database_postgresql_instance_credentials_reset", httpMethodPost, dbPostgreSQLInstancesPath, "credentials/reset",
			func(ctx context.Context, c *linode.Client, id int) (any, error) {
				return c.GetDatabasePostgreSQLInstance(ctx, id)
			})
	}

	if result := RequireConfirm(request, "This resets PostgreSQL Managed Database credentials. Set confirm=true to proceed."); result != nil {
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

	if err := client.ResetDatabasePostgreSQLInstanceCredentials(ctx, instanceID); err != nil {
		return mcp.NewToolResultError(formatDatabasePostgreSQLInstanceCredentialsResetError(err)), nil
	}

	response := struct {
		Message    string `json:"message"`
		InstanceID int    `json:"instance_id"`
	}{
		Message:    "PostgreSQL Managed Database credentials reset",
		InstanceID: instanceID,
	}

	return MarshalToolResponse(response)
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

func handleDatabaseTypesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := databaseTypesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	types, err := client.ListDatabaseTypes(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database types: %v", err)), nil
	}

	return FormatListResponse(types, nil, "database_types")
}

func handleDatabaseTypeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	typeID, validationMessage := databaseTypeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := databaseTypesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	databaseType, err := client.GetDatabaseType(ctx, typeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database type: %v", err)), nil
	}

	return MarshalToolResponse(databaseType)
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

	return FormatListResponse(instances, nil, responseKeyDatabaseInstances)
}

func handleDatabaseAllInstancesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := databaseInstancesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instances, err := client.ListAllDatabaseInstances(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Managed Database instances across engines: %v", err)), nil
	}

	return FormatListResponse(instances, nil, responseKeyDatabaseInstances)
}

func handleDatabasePostgreSQLInstancesListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := databaseInstancesPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instances, err := client.ListDatabasePostgreSQLInstances(ctx, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve PostgreSQL Managed Database instances: %v", err)), nil
	}

	return FormatListResponse(instances, nil, responseKeyDatabaseInstances)
}

// runDatabaseInstanceCreate validates create args, previews on dry_run
// (nil-fetch POST, current_state null since the resource does not exist
// yet), then gates on confirm and creates. Shared by the MySQL and
// PostgreSQL create handlers to stay under the dupl threshold.
func runDatabaseInstanceCreate(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, instancesPath, confirmMessage, messagePrefix string,
	create func(context.Context, *linode.Client, *linode.CreateDatabaseInstanceRequest) (*linode.DatabaseInstance, error),
) (*mcp.CallToolResult, error) {
	req, validationMessage := databaseInstanceCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, toolName, httpMethodPost, instancesPath, nil)
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	instance, err := create(ctx, client, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create %s instance: %v", messagePrefix, err)), nil
	}

	response := struct {
		Message  string                   `json:"message"`
		Instance *linode.DatabaseInstance `json:"database_instance"`
	}{
		Message:  fmt.Sprintf("%s instance '%s' (ID: %d) created", messagePrefix, instance.Label, instance.ID),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

func handleDatabaseInstanceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceCreate(ctx, request, cfg,
		"linode_database_mysql_instance_create", dbMySQLInstancesPath,
		"This creates a billable Managed Database instance. Set confirm=true to proceed.",
		dbMessagePrefixMySQL,
		func(ctx context.Context, c *linode.Client, req *linode.CreateDatabaseInstanceRequest) (*linode.DatabaseInstance, error) {
			return c.CreateDatabaseInstance(ctx, req)
		})
}

func handleDatabasePostgreSQLInstanceCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceCreate(ctx, request, cfg,
		"linode_database_postgresql_instance_create", dbPostgreSQLInstancesPath,
		"This creates a billable PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		dbMessagePrefixPostgreSQL,
		func(ctx context.Context, c *linode.Client, req *linode.CreateDatabaseInstanceRequest) (*linode.DatabaseInstance, error) {
			return c.CreateDatabasePostgreSQLInstance(ctx, req)
		})
}

func handleDatabaseInstanceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleDatabaseInstanceUpdateRequestWithClient(
		ctx,
		request,
		cfg,
		"linode_database_mysql_instance_update",
		dbMySQLInstancesPath,
		"This updates a Managed Database instance. Set confirm=true to proceed.",
		func(ctx context.Context, client *linode.Client, instanceID int) (any, error) {
			return client.GetDatabaseInstance(ctx, instanceID)
		},
		func(ctx context.Context, client *linode.Client, instanceID int, req *linode.UpdateDatabaseInstanceRequest) (*linode.DatabaseInstance, error) {
			return client.UpdateDatabaseInstance(ctx, instanceID, req)
		},
		dbMessagePrefixMySQL,
		formatDatabaseInstanceUpdateError,
	)
}

func handleDatabasePostgreSQLInstanceUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return handleDatabaseInstanceUpdateRequestWithClient(
		ctx,
		request,
		cfg,
		"linode_database_postgresql_instance_update",
		dbPostgreSQLInstancesPath,
		"This updates a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		func(ctx context.Context, client *linode.Client, instanceID int) (any, error) {
			return client.GetDatabasePostgreSQLInstance(ctx, instanceID)
		},
		func(ctx context.Context, client *linode.Client, instanceID int, req *linode.UpdateDatabaseInstanceRequest) (*linode.DatabaseInstance, error) {
			return client.UpdateDatabasePostgreSQLInstance(ctx, instanceID, req)
		},
		dbMessagePrefixPostgreSQL,
		formatDatabasePostgreSQLInstanceUpdateError,
	)
}

func handleDatabaseInstanceUpdateRequestWithClient(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, instancesPath, confirmMessage string,
	fetchState func(context.Context, *linode.Client, int) (any, error),
	update func(context.Context, *linode.Client, int, *linode.UpdateDatabaseInstanceRequest) (*linode.DatabaseInstance, error),
	messagePrefix string,
	formatError func(error) string,
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		instanceID, validationMessage := databaseInstanceIDFromTool(request)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, toolName, "PUT",
			fmt.Sprintf(instancesPath+"/%d", instanceID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return fetchState(ctx, c, instanceID)
			})
	}

	if result := RequireConfirm(request, confirmMessage); result != nil {
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

	instance, err := update(ctx, client, instanceID, req)
	if err != nil {
		return mcp.NewToolResultError(formatError(err)), nil
	}

	response := struct {
		Message  string                   `json:"message"`
		Instance *linode.DatabaseInstance `json:"database_instance"`
	}{
		Message:  fmt.Sprintf("%s instance '%s' (ID: %d) updated", messagePrefix, instance.Label, instance.ID),
		Instance: instance,
	}

	return MarshalToolResponse(response)
}

func handleDatabaseInstanceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// Pre-validation preserves the tool-specific "must be a positive
	// integer" message and rejects negatives, which the WithID helper's
	// default `== 0` check would not catch.
	if _, validationMessage := databaseInstanceIDFromTool(request); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_database_mysql_instance_delete",
		IDParam:        paramDatabaseInstanceID,
		Method:         httpMethodDelete,
		PathPattern:    dbMySQLInstancesPath + "/%d",
		ConfirmMessage: "This deletes a Managed Database instance. Set confirm=true to proceed.",
		SuccessFormat:  "Managed Database instance %d deleted",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteDatabaseInstance(ctx, id)
		},
		HashIgnore: twostage.HashIgnoreFields("DatabaseInstance"),
	})
}

func handleDatabasePostgreSQLInstanceDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if _, validationMessage := databaseInstanceIDFromTool(request); validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_database_postgresql_instance_delete",
		IDParam:        paramDatabaseInstanceID,
		Method:         httpMethodDelete,
		PathPattern:    dbPostgreSQLInstancesPath + "/%d",
		ConfirmMessage: "This deletes a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		SuccessFormat:  "PostgreSQL Managed Database instance %d deleted",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.DeleteDatabasePostgreSQLInstance(ctx, id)
		},
		HashIgnore: twostage.HashIgnoreFields("DatabaseInstance"),
	})
}

// runDatabaseInstanceActionDryRun previews an instance-scoped database
// action (patch/suspend/resume, the credentials GET, and the credentials
// reset) without firing it. current_state is the instance itself, never
// any credential the action might return or rotate, so the preview stays
// credential-safe. verb is the path suffix (e.g. "patch",
// "credentials/reset").
func runDatabaseInstanceActionDryRun(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, instancesPath, verb string,
	fetchState func(context.Context, *linode.Client, int) (any, error),
) (*mcp.CallToolResult, error) {
	instanceID, validationMessage := databaseInstanceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDryRunPreview(ctx, request, cfg, toolName, method,
		fmt.Sprintf(instancesPath+"/%d/"+verb, instanceID),
		func(ctx context.Context, c *linode.Client) (any, error) {
			return fetchState(ctx, c, instanceID)
		})
}

// databaseInstanceActionSpec describes an instance-scoped POST action
// (patch/suspend/resume) so the dry-run + confirm + execute flow lives in
// one place. Without this, the MySQL and PostgreSQL handler pairs are
// near-identical and trip the dupl linter once the dry-run branch lands.
type databaseInstanceActionSpec struct {
	ToolName       string
	InstancesPath  string
	Verb           string
	ConfirmMessage string
	MessagePrefix  string
	FetchState     func(context.Context, *linode.Client, int) (any, error)
	Execute        func(context.Context, *linode.Client, int) error
	FormatError    func(int, error) string
}

func runDatabaseInstanceAction(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, spec *databaseInstanceActionSpec) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return runDatabaseInstanceActionDryRun(ctx, request, cfg, spec.ToolName, httpMethodPost, spec.InstancesPath, spec.Verb, spec.FetchState)
	}

	if result := RequireConfirm(request, spec.ConfirmMessage); result != nil {
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

	if err := spec.Execute(ctx, client, instanceID); err != nil {
		return mcp.NewToolResultError(spec.FormatError(instanceID, err)), nil
	}

	return MarshalToolResponse(map[string]any{
		responseKeyMessage:      spec.MessagePrefix + " instance " + strconv.Itoa(instanceID) + " " + spec.Verb + " started",
		paramDatabaseInstanceID: instanceID,
	})
}

func handleDatabaseInstancePatchRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_mysql_instance_patch",
		InstancesPath:  dbMySQLInstancesPath,
		Verb:           "patch",
		ConfirmMessage: "This patches a Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixMySQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.PatchDatabaseInstance(ctx, id)
		},
		FormatError: formatDatabaseInstancePatchError,
	})
}

func handleDatabasePostgreSQLInstancePatchRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_postgresql_instance_patch",
		InstancesPath:  dbPostgreSQLInstancesPath,
		Verb:           "patch",
		ConfirmMessage: "This patches a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixPostgreSQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.PatchDatabasePostgreSQLInstance(ctx, id)
		},
		FormatError: formatDatabasePostgreSQLInstancePatchError,
	})
}

func handleDatabaseInstanceSuspendRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_mysql_instance_suspend",
		InstancesPath:  dbMySQLInstancesPath,
		Verb:           "suspend",
		ConfirmMessage: "This suspends a Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixMySQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.SuspendDatabaseInstance(ctx, id)
		},
		FormatError: formatDatabaseInstanceSuspendError,
	})
}

func handleDatabasePostgreSQLInstanceSuspendRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_postgresql_instance_suspend",
		InstancesPath:  dbPostgreSQLInstancesPath,
		Verb:           "suspend",
		ConfirmMessage: "This suspends a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixPostgreSQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.SuspendDatabasePostgreSQLInstance(ctx, id)
		},
		FormatError: formatDatabasePostgreSQLInstanceSuspendError,
	})
}

func handleDatabaseInstanceResumeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_mysql_instance_resume",
		InstancesPath:  dbMySQLInstancesPath,
		Verb:           "resume",
		ConfirmMessage: "This resumes a Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixMySQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabaseInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.ResumeDatabaseInstance(ctx, id)
		},
		FormatError: formatDatabaseInstanceResumeError,
	})
}

func handleDatabasePostgreSQLInstanceResumeRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return runDatabaseInstanceAction(ctx, request, cfg, &databaseInstanceActionSpec{
		ToolName:       "linode_database_postgresql_instance_resume",
		InstancesPath:  dbPostgreSQLInstancesPath,
		Verb:           "resume",
		ConfirmMessage: "This resumes a PostgreSQL Managed Database instance. Set confirm=true to proceed.",
		MessagePrefix:  dbMessagePrefixPostgreSQL,
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetDatabasePostgreSQLInstance(ctx, id)
		},
		Execute: func(ctx context.Context, c *linode.Client, id int) error {
			return c.ResumeDatabasePostgreSQLInstance(ctx, id)
		},
		FormatError: formatDatabasePostgreSQLInstanceResumeError,
	})
}

func formatDatabaseInstancePatchError(instanceID int, err error) string {
	return "Failed to patch Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
}

func formatDatabasePostgreSQLInstancePatchError(instanceID int, err error) string {
	return "Failed to patch PostgreSQL Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
}

func formatDatabaseInstanceSuspendError(instanceID int, err error) string {
	return "Failed to suspend Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
}

func formatDatabasePostgreSQLInstanceSuspendError(instanceID int, err error) string {
	return "Failed to suspend PostgreSQL Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
}

func formatDatabaseInstanceResumeError(instanceID int, err error) string {
	return "Failed to resume Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
}

func formatDatabasePostgreSQLInstanceResumeError(instanceID int, err error) string {
	return "Failed to resume PostgreSQL Managed Database instance " + strconv.Itoa(instanceID) + ": " + err.Error()
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

func databaseTypesPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", databaseTypesPageSizeMin, databaseTypesPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

func databaseTypeIDFromTool(request *mcp.CallToolRequest) (string, string) {
	typeID, ok := request.GetArguments()[paramDatabaseTypeID].(string)
	if !ok || strings.TrimSpace(typeID) == "" {
		return "", "type_id must be a non-empty string"
	}

	if typeID != strings.TrimSpace(typeID) || strings.Contains(typeID, "/") || strings.Contains(typeID, "?") || strings.Contains(typeID, "#") || strings.Contains(typeID, "..") {
		return "", "type_id must not contain separators, query, fragment, or traversal segments"
	}

	return typeID, ""
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

func formatDatabasePostgreSQLInstanceUpdateError(err error) string {
	return "Failed to update PostgreSQL Managed Database instance: " + err.Error()
}

func formatDatabasePostgreSQLInstanceCredentialsResetError(err error) string {
	return "Failed to reset PostgreSQL Managed Database credentials: " + err.Error()
}

func databaseInstanceUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateDatabaseInstanceRequest, string) {
	args := request.GetArguments()
	req := &linode.UpdateDatabaseInstanceRequest{}

	var changed bool

	allowList, hasAllowList, validationMessage := optionalAllowListFromTool(args)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if hasAllowList {
		req.AllowList = &allowList
		changed = true
	}

	engineConfig, hasEngineConfig, validationMessage := optionalMapJSONField(args, paramDatabaseEngineConfig)
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

	privateNetwork, hasPrivateNetwork, validationMessage := optionalMapJSONField(args, paramDatabasePrivateNetwork)
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

	updates, hasUpdates, validationMessage := optionalMapJSONField(args, paramDatabaseUpdates)
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

	allowList, hasAllowList, validationMessage := optionalAllowListFromTool(args)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	if hasAllowList {
		req.AllowList = allowList
	}

	if clusterSizeValue, ok := args[paramDatabaseClusterSize]; ok {
		clusterSize, ok := numberArgToInt(clusterSizeValue)
		if !ok || clusterSize < 1 {
			return linode.CreateDatabaseInstanceRequest{}, "cluster_size must be a positive integer"
		}

		req.ClusterSize = clusterSize
	}

	engineConfig, hasEngineConfig, validationMessage := optionalMapJSONField(args, paramDatabaseEngineConfig)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	if hasEngineConfig {
		req.EngineConfig = engineConfig
	}

	fork, hasFork, validationMessage := optionalMapJSONField(args, paramDatabaseFork)
	if validationMessage != "" {
		return linode.CreateDatabaseInstanceRequest{}, validationMessage
	}

	if hasFork {
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

// optionalAllowListFromTool reads the optional native "allow_list" array. It also
// accepts a JSON-encoded string from a non-compliant client.
func optionalAllowListFromTool(args map[string]any) ([]string, bool, string) {
	raw, present := args[paramDatabaseAllowList]
	if !present {
		return nil, false, ""
	}

	values, validationMessage := stringSliceFromToolArg(raw, paramDatabaseAllowList)
	if validationMessage != "" {
		return nil, false, validationMessage
	}

	return values, true, ""
}

// optionalMapJSONField reads an optional object argument as a native map (the
// schema form) or a JSON-encoded object string (legacy form). An absent or empty
// value yields (nil, false, ""); a non-object value returns a validation message.
func optionalMapJSONField(args map[string]any, key string) (map[string]any, bool, string) {
	raw, present := args[key]
	if !present {
		return nil, false, ""
	}

	objectError := key + " must be an object"

	switch value := raw.(type) {
	case map[string]any:
		return value, true, ""
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, false, ""
		}

		var values map[string]any
		if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
			return nil, false, objectError
		}

		if values == nil {
			return nil, false, objectError
		}

		return values, true, ""
	default:
		return nil, false, objectError
	}
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
