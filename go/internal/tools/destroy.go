package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// Shared literals for destroy-tool response building. Extracted so
// per-tool struct literals don't each repeat them (which trips
// goconst once enough tools accumulate the same string).
const (
	httpMethodDelete   = "DELETE"
	responseKeyMessage = "message"
)

// DestructiveAction packages per-tool customization for the destroy
// flow. Caller-built path; closures over caller-parsed IDs. Used
// directly by tools with multi-ID paths (e.g. domain-record delete
// nested under a domain). Single-ID destroys should reach for
// RunDestructiveActionWithID instead, which is short enough per
// caller to stay below the dupl linter's threshold.
type DestructiveAction struct {
	ToolName       string
	Method         string
	Path           string
	ConfirmMessage string
	FetchState     func(ctx context.Context, client *linode.Client) (any, error)
	Execute        func(ctx context.Context, client *linode.Client) error
	Success        func() any
}

// RunDestructiveAction runs the shared destroy-tool flow: prepare the
// client, branch on dry-run vs real execution. On dry-run: fetch state
// and emit the v0 preview. On real: confirm gate, execute, build
// success response. Per-tool validation happens in the caller.
func RunDestructiveAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if IsDryRun(request) {
		state, fetchErr := action.FetchState(ctx, client)
		if fetchErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch state for dry-run: %v", fetchErr)), nil
		}

		return BuildDryRunResponse(
			action.ToolName,
			request.GetString(paramEnvironment, ""),
			action.Method,
			action.Path,
			state,
		)
	}

	if result := RequireConfirm(request, action.ConfirmMessage); result != nil {
		return result, nil
	}

	if execErr := action.Execute(ctx, client); execErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s failed: %v", action.ToolName, execErr)), nil
	}

	return MarshalToolResponse(action.Success())
}

// DestructiveActionByID configures a single-ID destroy tool. The vast
// majority of CapDestroy tools take one integer ID arg, a fixed path
// shape, and a uniform success response, so this thinner config form
// keeps each per-tool handler short (around 10 lines) and well below
// the dupl linter's threshold.
//
// FetchState and Execute take the parsed ID directly, sidestepping the
// closure capture each caller would otherwise need.
type DestructiveActionByID struct {
	ToolName       string
	IDParam        string // request arg name, e.g. "domain_id"
	Method         string
	PathPattern    string // single %d slot for the ID, e.g. "/domains/%d"
	ConfirmMessage string
	SuccessFormat  string // single %d slot for the ID, e.g. "Domain %d removed successfully"
	FetchState     func(ctx context.Context, client *linode.Client, id int) (any, error)
	Execute        func(ctx context.Context, client *linode.Client, id int) error
}

// RunDestructiveActionWithID is the single-ID convenience wrapper over
// RunDestructiveAction. It parses and validates the integer ID arg
// named by config.IDParam, then delegates to the underlying flow with
// closures that capture the parsed ID. Per-tool handlers reduce to a
// single struct literal, which keeps them under dupl's threshold.
func RunDestructiveActionWithID(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	params *DestructiveActionByID,
) (*mcp.CallToolResult, error) {
	id := request.GetInt(params.IDParam, 0)
	if id == 0 {
		return mcp.NewToolResultError(params.IDParam + " is required"), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       params.ToolName,
		Method:         params.Method,
		Path:           fmt.Sprintf(params.PathPattern, id),
		ConfirmMessage: params.ConfirmMessage,
		FetchState: func(ctx context.Context, client *linode.Client) (any, error) {
			return params.FetchState(ctx, client, id)
		},
		Execute: func(ctx context.Context, client *linode.Client) error {
			return params.Execute(ctx, client, id)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf(params.SuccessFormat, id),
				params.IDParam:     id,
			}
		},
	})
}
