package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	transferKeyYear  = "year"
	transferKeyMonth = "month"
	transferMonthMax = 12
)

// NewLinodeInstanceTransferMonthGetTool creates a tool for retrieving monthly network transfer statistics for a Linode instance.
func NewLinodeInstanceTransferMonthGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_transfer_month_get",
		"Retrieves network transfer statistics for a Linode instance for a specific month.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber(transferKeyYear, mcp.Required(),
				mcp.Description("The year for the transfer stats, for example 2024")),
			mcp.WithNumber(transferKeyMonth, mcp.Required(),
				mcp.Description("The month for the transfer stats, 1 through 12")),
		},
		handleInstanceTransferMonthGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceTransferMonthGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := requiredPositiveToolInt(request, "linode_id", "linode_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	year, validationMessage := requiredPositiveToolInt(request, transferKeyYear, transferKeyYear)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	month, validationMessage := requiredToolIntInRange(request, transferKeyMonth, transferKeyMonth, 1, transferMonthMax)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	transfer, err := client.GetInstanceTransferByYearMonth(ctx, linodeID, year, month)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve transfer stats for instance %d in %04d-%02d: %v", linodeID, year, month, err)), nil
	}

	return MarshalToolResponse(transfer)
}

func requiredPositiveToolInt(request *mcp.CallToolRequest, key, label string) (int, string) {
	return requiredToolIntInRange(request, key, label, 1, 0)
}

func requiredToolIntInRange(request *mcp.CallToolRequest, key, label string, minValue, maxValue int) (int, string) {
	args := request.GetArguments()
	if _, exists := args[key]; !exists {
		return 0, label + " is required"
	}

	value, validationMessage := optionalPaginationInt(args, key, minValue, maxValue)
	if validationMessage != "" {
		return 0, validationMessage
	}

	return value, ""
}
