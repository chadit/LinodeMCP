package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	profileTokensPath       = "/profile/tokens"
	profileTokenFieldExpiry = "expiry"
	profileTokenFieldLabel  = "label"
	profileTokenFieldScopes = "scopes"
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

// NewLinodeProfileTokenCreateTool creates a tool for creating a personal access token.
func NewLinodeProfileTokenCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_token_create",
		"Creates a personal access token for the authenticated profile. Pass dry_run=true to preview without creating a token.",
		[]mcp.ToolOption{
			mcp.WithString("expiry", mcp.Description("Token expiry timestamp (optional).")),
			mcp.WithString("label", mcp.Description("Token label (optional).")),
			mcp.WithString("scopes", mcp.Description("Token scopes string (optional).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm personal access token creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTokenCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeProfileTokenCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	body, validationMessage := profileTokenCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreviewWithBody(ctx, request, cfg, "linode_profile_token_create", httpMethodPost, profileTokensPath, body, nil)
	}

	if result := RequireConfirm(request, "This creates a personal access token. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	token, createFailureMessage := createProfileTokenResult(ctx, client, body)
	if createFailureMessage != "" {
		return mcp.NewToolResultError(createFailureMessage), nil
	}

	return MarshalToolResponse(token)
}

func createProfileTokenResult(ctx context.Context, client *linode.Client, body linode.CreateProfileTokenRequest) (*linode.ProfileToken, string) {
	token, createFailure := client.CreateProfileToken(ctx, body)
	if createFailure != nil {
		return nil, "Failed to create linode_profile_token_create: " + createFailure.Error()
	}

	return token, ""
}

func profileTokenCreateRequestFromTool(request *mcp.CallToolRequest) (linode.CreateProfileTokenRequest, string) {
	args := request.GetArguments()
	body := linode.CreateProfileTokenRequest{}

	if value, ok := args["expiry"]; ok {
		expiry, valid := value.(string)
		if !valid {
			return body, "expiry must be a string"
		}

		body.Expiry = expiry
	}

	if value, ok := args["label"]; ok {
		label, valid := value.(string)
		if !valid {
			return body, "label must be a string"
		}

		body.Label = label
	}

	if value, ok := args["scopes"]; ok {
		scopes, valid := value.(string)
		if !valid {
			return body, "scopes must be a string"
		}

		body.Scopes = scopes
	}

	return body, ""
}

// NewLinodeProfileTokenDeleteTool creates a tool for revoking a personal access token.
func NewLinodeProfileTokenDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_token_delete",
		"Revokes a personal access token for the authenticated profile. Pass dry_run=true to preview without revoking the token.",
		[]mcp.ToolOption{
			mcp.WithNumber("token_id", mcp.Required(), mcp.Description("The personal access token ID to revoke.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm personal access token revocation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTokenDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeProfileTokenDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenID, validationMessage := requiredPositiveToolInt(request, "token_id", "token_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	path := fmt.Sprintf("%s/%d", profileTokensPath, tokenID)

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_profile_token_delete", httpMethodDelete, path, nil)
	}

	if result := RequireConfirm(request, "This revokes a personal access token. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, prepErr := prepareClient(request, cfg)
	if prepErr != nil {
		return mcp.NewToolResultError(prepErr.Error()), nil
	}

	if deleteFailureMessage := deleteProfileTokenResult(ctx, client, tokenID); deleteFailureMessage != "" {
		return mcp.NewToolResultError(deleteFailureMessage), nil
	}

	return MarshalToolResponse(map[string]any{
		responseKeyMessage: "Profile token revoked successfully",
		"token_id":         tokenID,
	})
}

func deleteProfileTokenResult(ctx context.Context, client *linode.Client, tokenID int) string {
	if deleteFailure := client.DeleteProfileToken(ctx, tokenID); deleteFailure != nil {
		return "Failed to delete linode_profile_token_delete: " + deleteFailure.Error()
	}

	return ""
}

// NewLinodeProfileTokenUpdateTool creates a tool for updating a personal access token.
func NewLinodeProfileTokenUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_token_update",
		"Updates a personal access token for the authenticated profile.",
		[]mcp.ToolOption{
			mcp.WithNumber(profileTokenIDParam, mcp.Required(), mcp.Description("Personal access token ID.")),
			mcp.WithString(profileTokenFieldExpiry, mcp.Description("Expiry timestamp to send in the update body.")),
			mcp.WithString(profileTokenFieldLabel, mcp.Description("Token label.")),
			mcp.WithString(profileTokenFieldScopes, mcp.Description("Token scopes.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm updating a personal access token. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileTokenUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeProfileTokenUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	tokenID, body, validationMessage := profileTokenUpdateFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	tokenIDString := strconv.Itoa(tokenID)

	path := "/profile/tokens/" + tokenIDString
	if IsDryRun(request) {
		return RunDryRunPreviewWithBody(ctx, request, cfg, "linode_profile_token_update", httpMethodPut, path, body, nil)
	}

	if result := RequireConfirm(request, "This updates a personal access token. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	token, updateFailureMessage := updateProfileToken(ctx, client, tokenIDString, body)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update linode_profile_token_update: " + updateFailureMessage), nil
	}

	return MarshalToolResponse(token)
}

func updateProfileToken(ctx context.Context, client *linode.Client, tokenID string, body linode.UpdateProfileTokenRequest) (*linode.ProfileToken, string) {
	token, err := client.UpdateProfileToken(ctx, tokenID, body)
	if err != nil {
		return nil, err.Error()
	}

	return token, ""
}

func profileTokenUpdateFromTool(request *mcp.CallToolRequest) (int, linode.UpdateProfileTokenRequest, string) {
	tokenID, validationMessage := profileTokenIDFromTool(request)
	if validationMessage != "" {
		return 0, nil, validationMessage
	}

	body := linode.UpdateProfileTokenRequest{}

	args := request.GetArguments()
	for _, key := range []string{profileTokenFieldExpiry, profileTokenFieldLabel, profileTokenFieldScopes} {
		if raw, exists := args[key]; exists {
			value, ok := raw.(string)
			if !ok || value == "" {
				return 0, nil, key + " must be a non-empty string"
			}

			body[key] = value
		}
	}

	if len(body) == 0 {
		return 0, nil, "at least one profile " + "token field is required"
	}

	return tokenID, body, ""
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
