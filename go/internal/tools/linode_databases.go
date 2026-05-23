package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	paramDatabaseEngineID = "engine_id"

	databaseEnginesPageSizeMin = 25
	databaseEnginesPageSizeMax = 500
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
