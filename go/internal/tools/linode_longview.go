package tools

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeLongviewPlanTool creates a tool for retrieving the Longview subscription plan.
func NewLinodeLongviewPlanTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_plan",
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
		return mcp.NewToolResultError("Failed to retrieve linode_longview_plan: " + getFailureMessage), nil
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

// NewLinodeLongviewClientCreateTool creates a tool for creating a Longview client.
func NewLinodeLongviewClientCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_longview_client_create",
		"Creates a Longview client. WARNING: the API key and install code are returned in the response and may be needed to configure the client application.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(), mcp.Description("Label for the Longview client.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Longview client creation.")),
		},
		handleLinodeLongviewClientCreateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeLongviewClientCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This creates a Longview client and returns setup credentials. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := longviewClientCreateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
		"Updates the account Longview subscription plan.",
		[]mcp.ToolOption{
			mcp.WithString("longview_subscription", mcp.Required(), mcp.Description("Longview subscription plan slug to apply.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(), mcp.Description("Must be true to confirm Longview plan update.")),
		},
		handleLinodeLongviewPlanUpdateRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeLongviewPlanUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This updates the Longview subscription plan. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := longviewPlanUpdateRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
