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

// Common MCP tool parameter names and descriptions used across all tools.
const (
	paramEnvironment     = "environment"
	paramEnvironmentDesc = "Linode environment to use (optional, defaults to 'default')"
	paramConfirm         = "confirm"
)

// marshalToolResponse serializes v as indented JSON and wraps it in an MCP text result.
func marshalToolResponse(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

// prepareClient extracts the environment parameter, validates the config, and returns a ready-to-use API client.
func prepareClient(request *mcp.CallToolRequest, cfg *config.Config) (*linode.RetryableClient, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return nil, err
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return nil, err
	}

	return linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token), nil
}

// requireConfirm checks the confirm parameter and returns an error result if not set.
func requireConfirm(request *mcp.CallToolRequest, message string) *mcp.CallToolResult {
	if !request.GetBool(paramConfirm, false) {
		return mcp.NewToolResultError(message)
	}

	return nil
}

// newSimpleGetTool creates a tool that retrieves a single API resource with no parameters beyond environment.
func newSimpleGetTool(
	cfg *config.Config,
	toolName, description string,
	apiCall func(context.Context, *linode.RetryableClient) (any, error),
) (mcp.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(toolName,
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := apiCall(ctx, client)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve %s: %v", toolName, err)), nil
		}

		return marshalToolResponse(result)
	}

	return tool, handler
}

// filterByField returns items where getField matches filter (case-insensitive).
func filterByField[T any](items []T, filter string, getField func(T) string) []T {
	filtered := make([]T, 0, len(items))

	for i := range items {
		if strings.EqualFold(getField(items[i]), filter) {
			filtered = append(filtered, items[i])
		}
	}

	return filtered
}

// filterByContains returns items where getField contains substr (case-insensitive).
func filterByContains[T any](items []T, substr string, getField func(T) string) []T {
	filtered := make([]T, 0, len(items))

	lower := strings.ToLower(substr)

	for i := range items {
		if strings.Contains(strings.ToLower(getField(items[i])), lower) {
			filtered = append(filtered, items[i])
		}
	}

	return filtered
}

// formatListResponse builds a standard list response with count, optional filter, and items under the given JSON key.
func formatListResponse[T any](items []T, appliedFilters []string, key string) (*mcp.CallToolResult, error) {
	response := map[string]any{
		"count": len(items),
		key:     items,
	}

	if len(appliedFilters) > 0 {
		response["filter"] = strings.Join(appliedFilters, ", ")
	}

	return marshalToolResponse(response)
}

// filterDef describes a filter parameter for list tools.
type filterDef[T any] struct {
	paramName string
	matchFunc func(items []T, value string) []T
}

// handleListRequest is a generic handler for list-style tools that fetch items,
// apply filters, and format the response.
func handleListRequest[T any](
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	apiCall func(context.Context, *linode.RetryableClient) ([]T, error),
	filters []filterDef[T],
	formatResponse func(items []T, appliedFilters []string) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	items, err := apiCall(ctx, client)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
	}

	var appliedFilters []string

	for _, f := range filters {
		value := request.GetString(f.paramName, "")
		if value != "" {
			items = f.matchFunc(items, value)
			appliedFilters = append(appliedFilters, f.paramName+"="+value)
		}
	}

	return formatResponse(items, appliedFilters)
}

// listFilterParam combines a tool parameter definition with its filter logic for list tools.
type listFilterParam[T any] struct {
	paramName   string
	description string
	matchFunc   func(items []T, value string) []T
}

// fieldFilter creates a list filter parameter that matches a field exactly (case-insensitive).
func fieldFilter[T any](paramName, description string, getField func(T) string) listFilterParam[T] {
	return listFilterParam[T]{
		paramName:   paramName,
		description: description,
		matchFunc: func(items []T, value string) []T {
			return filterByField(items, value, getField)
		},
	}
}

// containsFilter creates a list filter parameter that matches a field by substring (case-insensitive).
func containsFilter[T any](paramName, description string, getField func(T) string) listFilterParam[T] {
	return listFilterParam[T]{
		paramName:   paramName,
		description: description,
		matchFunc: func(items []T, value string) []T {
			return filterByContains(items, value, getField)
		},
	}
}

// newListTool creates a complete list tool with filtering support using a generic factory pattern.
// It builds the MCP tool definition, filter pipeline, and handler from the provided parameters.
func newListTool[T any](
	cfg *config.Config,
	toolName, description string,
	apiCall func(context.Context, *linode.RetryableClient) ([]T, error),
	filterParams []listFilterParam[T],
	responseKey string,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	options := make([]mcp.ToolOption, 0, 2+len(filterParams))
	options = append(options,
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)

	filters := make([]filterDef[T], 0, len(filterParams))

	for _, fp := range filterParams {
		options = append(options, mcp.WithString(fp.paramName, mcp.Description(fp.description)))
		filters = append(filters, filterDef[T]{paramName: fp.paramName, matchFunc: fp.matchFunc})
	}

	tool := mcp.NewTool(toolName, options...)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListRequest(ctx, &request, cfg, apiCall, filters,
			func(items []T, appliedFilters []string) (*mcp.CallToolResult, error) {
				return formatListResponse(items, appliedFilters, responseKey)
			},
		)
	}

	return tool, handler
}

// toolHandlerFunc is the signature for tool handler functions that receive a pointer to the request.
type toolHandlerFunc func(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error)

// newToolWithHandler creates an MCP tool definition paired with a handler, factoring out the
// common boilerplate of adding the environment parameter and wrapping the handler closure.
func newToolWithHandler(
	cfg *config.Config,
	name, description string,
	options []mcp.ToolOption,
	handler toolHandlerFunc,
) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	allOptions := make([]mcp.ToolOption, 0, len(options)+2)
	allOptions = append(allOptions,
		mcp.WithDescription(description),
		mcp.WithString(paramEnvironment, mcp.Description(paramEnvironmentDesc)),
	)
	allOptions = append(allOptions, options...)

	tool := mcp.NewTool(name, allOptions...)

	wrappedHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handler(ctx, &request, cfg)
	}

	return tool, wrappedHandler
}
