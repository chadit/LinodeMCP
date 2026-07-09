package tools

import (
	"context"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const errLabel3To32Chars = "label must be 3 to 32 characters"

var (
	longviewClientLabelCharset      = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	longviewPlanSubscriptionPattern = regexp.MustCompile(`^longview-[1-9]\d*$`)
)

// NewLinodeLongviewPlanTool creates a tool for retrieving the Longview subscription plan.
func NewLinodeLongviewPlanTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_longview_plan_get",
		"Gets the current Longview subscription plan for the account.",
		toolschemas.Schema("linode.mcp.v1.LongviewPlanGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeLongviewPlanRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeLongviewPlanRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	plan, getFailure := client.GetLongviewPlanProto(ctx)
	if getFailure == nil {
		return MarshalProtoToolResponse(plan)
	}

	return mcp.NewToolResultError("Failed to retrieve linode_longview_plan_get: " + getFailure.Error()), nil
}

// NewLinodeLongviewTypesTool creates a tool for listing available Longview subscription types.
func NewLinodeLongviewTypesTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_longview_type_list",
		"Lists available Longview subscription types.",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LongviewType, error) {
			return client.ListLongviewTypesProto(ctx)
		},
		nil,
		longviewTypeListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_longview_type_list",
		"Lists available Longview subscription types.",
		toolschemas.Schema("linode.mcp.v1.LongviewTypeListInput"),
	)

	return tool, profiles.CapRead, handler
}

func longviewTypeListResponse(items []*linodev1.LongviewType, count int32, filter *string) *linodev1.LongviewTypeListResponse {
	return &linodev1.LongviewTypeListResponse{Count: count, Filter: filter, Types: items}
}

// NewLinodeLongviewSubscriptionsTool creates a tool for listing available Longview subscriptions.
func NewLinodeLongviewSubscriptionsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolPaginated(
		cfg,
		"linode_longview_subscription_list",
		"Lists available Longview subscription plans.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.LongviewSubscription, error) {
			return client.ListLongviewSubscriptionsProto(ctx, page, pageSize)
		},
		longviewSubscriptionsPaginationFromTool,
		nil,
		longviewSubscriptionListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_longview_subscription_list",
		"Lists available Longview subscription plans.",
		toolschemas.Schema("linode.mcp.v1.LongviewSubscriptionListInput"),
	)

	return tool, profiles.CapRead, handler
}

func longviewSubscriptionListResponse(items []*linodev1.LongviewSubscription, count int32, filter *string) *linodev1.LongviewSubscriptionListResponse {
	return &linodev1.LongviewSubscriptionListResponse{Count: count, Filter: filter, LongviewSubscriptions: items}
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
	tool := mcp.NewToolWithRawSchema(
		"linode_longview_client_create",
		"Creates a Longview client. WARNING: the API key and install code are returned in the response and may be needed to configure the client application. Pass dry_run=true to preview without creating.",
		toolschemas.Schema("linode.mcp.v1.LongviewClientCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeLongviewClientCreateRequest(ctx, &request, cfg)
	}

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

	return MarshalProtoToolResponse(&linodev1.LongviewClientCreateWriteResponse{
		Message:        "Longview client created successfully",
		Warning:        "IMPORTANT: Save the API key and install code if they are present; they are required to configure the Longview client application.",
		LongviewClient: longviewClient,
	})
}

func createLongviewClient(ctx context.Context, client *linode.Client, req *linode.CreateLongviewClientRequest) (*linodev1.CreatedLongviewClient, string) {
	longviewClient, err := client.CreateLongviewClientProto(ctx, req)
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

	// Length + charset checks ported from Python's _validate_longview_client_label
	// (Go create previously only required non-empty). Two messages match Python's.
	if len(label) < 3 || len(label) > 32 {
		return nil, errLabel3To32Chars
	}

	if !longviewClientLabelCharset.MatchString(label) {
		return nil, "label may only contain letters, numbers, hyphens, and underscores"
	}

	return &linode.CreateLongviewClientRequest{Label: label}, ""
}

// NewLinodeLongviewPlanUpdateTool creates a tool for updating the Longview subscription plan.
func NewLinodeLongviewPlanUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_longview_plan_update",
		"Updates the account Longview subscription plan. Pass dry_run=true to preview without modifying.",
		toolschemas.Schema("linode.mcp.v1.LongviewPlanUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeLongviewPlanUpdateRequest(ctx, &request, cfg)
	}

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

	return MarshalProtoToolResponse(&linodev1.LongviewSubscriptionWriteResponse{
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

	// Plan-id pattern ported from Python (Go previously accepted any string).
	if !longviewPlanSubscriptionPattern.MatchString(subscription) {
		return nil, "longview_subscription must be a Longview plan ID like longview-10"
	}

	return &linode.UpdateLongviewPlanRequest{LongviewSubscription: subscription}, ""
}

func updateLongviewPlan(ctx context.Context, client *linode.Client, req *linode.UpdateLongviewPlanRequest) (*linodev1.LongviewSubscription, string) {
	plan, err := client.UpdateLongviewPlanProto(ctx, req)
	if err != nil {
		return nil, err.Error()
	}

	return plan, ""
}
