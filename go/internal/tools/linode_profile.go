package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeProfileTool creates a tool for retrieving Linode profile info.
func NewLinodeProfileTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_profile",
		"Retrieves Linode user account profile information",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetProfile(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeProfilePreferencesTool creates a tool for retrieving Linode profile preferences.
func NewLinodeProfilePreferencesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_profile_preferences",
		"Retrieves Linode user preference settings",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetProfilePreferences(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeProfileSecurityQuestionsTool creates a tool for listing available profile security questions.
func NewLinodeProfileSecurityQuestionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newSimpleGetTool(
		cfg, "linode_profile_security_questions",
		"Lists available profile security questions for the authenticated profile",
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.ListProfileSecurityQuestions(ctx)
		},
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeProfilePreferencesUpdateTool creates a tool for updating profile preferences.
func NewLinodeProfilePreferencesUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_preferences_update",
		"Updates dashboard preferences for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithObject("preferences", mcp.Required(),
				mcp.Description("Preference fields to send as the JSON request body.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm updating profile preferences. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfilePreferencesUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeProfilePreferencesUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	body, validationMessage := profilePreferencesFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBody(ctx, request, cfg, "linode_profile_preferences_update", httpMethodPut, "/profile/preferences", body, nil)
	}

	if result := RequireConfirm(request, "This updates profile preferences. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	preferences, updateFailureMessage := updateProfilePreferencesResult(ctx, client, body)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError(updateFailureMessage), nil
	}

	return MarshalToolResponse(preferences)
}

func updateProfilePreferencesResult(ctx context.Context, client *linode.Client, body linode.ProfilePreferences) (linode.ProfilePreferences, string) {
	preferences, updateFailure := client.UpdateProfilePreferences(ctx, body)
	if updateFailure != nil {
		return nil, "Failed to update linode_profile_preferences_update: " + updateFailure.Error()
	}

	return preferences, ""
}

func profilePreferencesFromTool(request *mcp.CallToolRequest) (linode.ProfilePreferences, string) {
	raw, ok := request.GetArguments()["preferences"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, "preferences must be a non-empty object"
	}

	return linode.ProfilePreferences(raw), ""
}

// NewLinodeProfileTokensTool creates a tool for listing personal access tokens for the authenticated profile.
func NewLinodeProfileTokensTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_tokens",
		"Lists personal access tokens for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeProfileTokensRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeProfileTokensRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := profileTokensPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tokens, listFailure := client.ListProfileTokens(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(tokens)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_tokens: " + listFailure.Error()), nil
}

func profileTokensPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountLoginsPageSizeMin, accountLoginsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// NewLinodeProfileLoginsTool creates a tool for listing login history for the authenticated profile.
func NewLinodeProfileLoginsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_logins",
		"Lists login history for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeProfileLoginsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeProfileLoginsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := profileLoginsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	logins, listFailure := client.ListProfileLogins(ctx, page, pageSize)
	if listFailure == nil {
		return MarshalToolResponse(logins)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_profile_logins: " + listFailure.Error()), nil
}

func profileLoginsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", accountLoginsPageSizeMin, accountLoginsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}
