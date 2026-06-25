package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeLongviewPlanTool creates a tool for retrieving the Longview subscription plan.
func NewLinodeLongviewPlanTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_plan_get",
		"Gets the current Longview subscription plan for the account.",
		nil,
		handleLinodeLongviewPlanRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeLongviewPlanRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	plan, getFailureMessage := getLongviewPlan(ctx, client)
	if getFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_longview_plan_get: " + getFailureMessage), nil
	}

	return MarshalToolResponse(plan)
}

func getLongviewPlan(ctx context.Context, client *linode.Client) (*linode.LongviewSubscription, string) {
	plan, err := client.GetLongviewPlan(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return plan, ""
}

// NewLinodeLongviewTypesTool creates a tool for listing available Longview subscription types.
func NewLinodeLongviewTypesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_type_list",
		"Lists available Longview subscription types.",
		nil,
		handleLinodeLongviewTypesRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeLongviewTypesRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	types, listFailureMessage := listLongviewTypes(ctx, client)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_longview_type_list: " + listFailureMessage), nil
	}

	return MarshalToolResponse(types)
}

func listLongviewTypes(ctx context.Context, client *linode.Client) (*linode.PaginatedResponse[linode.LongviewType], string) {
	types, err := client.ListLongviewTypes(ctx)
	if err != nil {
		return nil, err.Error()
	}

	return types, ""
}

// NewLinodeLongviewSubscriptionsTool creates a tool for listing available Longview subscriptions.
func NewLinodeLongviewSubscriptionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_subscription_list",
		"Lists available Longview subscription plans.",
		[]mcp.ToolOption{
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleLinodeLongviewSubscriptionsRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeLongviewSubscriptionsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	page, pageSize, validationMessage := longviewSubscriptionsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	subscriptions, listFailureMessage := listLongviewSubscriptions(ctx, client, page, pageSize)
	if listFailureMessage != "" {
		return mcp.NewToolResultError("Failed to retrieve linode_longview_subscription_list: " + listFailureMessage), nil
	}

	return MarshalToolResponse(subscriptions)
}

func listLongviewSubscriptions(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.LongviewSubscription], string) {
	subscriptions, err := client.ListLongviewSubscriptions(ctx, page, pageSize)
	if err != nil {
		return nil, err.Error()
	}

	return subscriptions, ""
}

func longviewSubscriptionsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", longviewSubscriptionsPageSizeMin, longviewSubscriptionsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

// NewLinodeLongviewClientCreateTool creates a tool for creating a Longview client.
func NewLinodeLongviewClientCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_client_create",
		"Creates a Longview client. WARNING: the API key and install code are returned in the response and may be needed to configure the client application. Pass dry_run=true to preview without creating.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(), mcp.Description("Label for the Longview client.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Longview client creation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeLongviewClientCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeLongviewClientCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := longviewClientCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_longview_client_create", httpMethodPost, longviewClientsPath, nil)
	}

	if result := RequireConfirm(request, "This creates a Longview client and returns setup credentials. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	longviewClient, createFailureMessage := createLongviewClient(ctx, client, req)
	if createFailureMessage != "" {
		return mcp.NewToolResultError("Failed to create linode_longview_client_create: " + createFailureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message string                        `json:"message"`
		Warning string                        `json:"warning"`
		Client  *linode.CreatedLongviewClient `json:"client"`
	}{
		Message: "Longview client created successfully",
		Warning: "IMPORTANT: Save the API key and install code if they are present; they are required to configure the Longview client application.",
		Client:  longviewClient,
	})
}

func createLongviewClient(ctx context.Context, client *linode.Client, req *linode.CreateLongviewClientRequest) (*linode.CreatedLongviewClient, string) {
	longviewClient, err := client.CreateLongviewClient(ctx, req)
	if err != nil {
		return nil, err.Error()
	}

	return longviewClient, ""
}

func longviewClientCreateRequestFromTool(request *mcp.CallToolRequest) (*linode.CreateLongviewClientRequest, string) {
	label, ok := request.GetArguments()["label"].(string)
	if !ok {
		return nil, errLabelRequired
	}

	label = strings.TrimSpace(label)
	if label == "" {
		return nil, errLabelRequired
	}

	return &linode.CreateLongviewClientRequest{Label: label}, ""
}

// NewLinodeLongviewPlanUpdateTool creates a tool for updating the Longview subscription plan.
func NewLinodeLongviewPlanUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_plan_update",
		"Updates the account Longview subscription plan. Pass dry_run=true to preview without modifying.",
		[]mcp.ToolOption{
			mcp.WithString("longview_subscription", mcp.Required(), mcp.Description("Longview subscription plan slug to apply.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Longview plan update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeLongviewPlanUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeLongviewPlanUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := longviewPlanUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_longview_plan_update", "PUT", longviewPlanPath,
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetLongviewPlan(ctx)
			})
	}

	if result := RequireConfirm(request, "This updates the Longview subscription plan. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	plan, updateFailureMessage := updateLongviewPlan(ctx, client, req)
	if updateFailureMessage != "" {
		return mcp.NewToolResultError("Failed to update linode_longview_plan_update: " + updateFailureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message string                       `json:"message"`
		Plan    *linode.LongviewSubscription `json:"plan"`
	}{
		Message: "Longview plan updated successfully",
		Plan:    plan,
	})
}

func longviewPlanUpdateRequestFromTool(request *mcp.CallToolRequest) (*linode.UpdateLongviewPlanRequest, string) {
	subscription, ok := request.GetArguments()["longview_subscription"].(string)
	if !ok {
		return nil, "longview_subscription is required"
	}

	subscription = strings.TrimSpace(subscription)
	if subscription == "" {
		return nil, "longview_subscription is required"
	}

	return &linode.UpdateLongviewPlanRequest{LongviewSubscription: subscription}, ""
}

func updateLongviewPlan(ctx context.Context, client *linode.Client, req *linode.UpdateLongviewPlanRequest) (*linode.LongviewSubscription, string) {
	plan, err := client.UpdateLongviewPlan(ctx, req)
	if err != nil {
		return nil, err.Error()
	}

	return plan, ""
}
