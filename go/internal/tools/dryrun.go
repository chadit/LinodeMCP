package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// DryRunResponse is the v0 wire shape returned by mutating tools when
// invoked with dry_run:true. Phase 1 ships this with the top-level
// fields populated; Phase 2 per-tool dependency walks elevate it with
// Dependencies, BillingDelta, and Warnings.
//
// The struct field set is intentionally small in Phase 1 so the wire
// shape stays stable as Phase 2 lands. Clients (including the model)
// must treat unset optional fields as "not provided", not "empty".
type DryRunResponse struct {
	DryRun       bool          `json:"dry_run"`
	Tool         string        `json:"tool"`
	Environment  string        `json:"environment,omitempty"`
	WouldExecute DryRunRequest `json:"would_execute"`
	CurrentState any           `json:"current_state"`

	// Phase 2 enrichment, all optional (omitempty). Phase 1 tools and
	// Tier C tools leave these unset, keeping the v0 wire shape stable
	// across every tool wired before Phase 2.
	Dependencies []DryRunDependency  `json:"dependencies,omitempty"`
	SideEffects  []string            `json:"side_effects,omitempty"`
	BillingDelta *DryRunBillingDelta `json:"billing_delta,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// DryRunDependency is one resource a destructive call would affect. The
// dependency walk for a Tier A tool returns a slice of these. Kind names
// the resource type (e.g. "volume", "public_ip", "nodebalancer_backend").
// ID is an int or string identifier (omitted for resources without a
// stable id, like a released IP carried in Label). Action is one of
// detached, released, removed, cascade_deleted. Note is free-form context.
type DryRunDependency struct {
	Kind   string `json:"kind"`
	ID     any    `json:"id,omitempty"`
	Label  string `json:"label,omitempty"`
	Action string `json:"action"`
	Note   string `json:"note,omitempty"`
}

// DryRunBillingDelta is a best-effort monthly cost change estimate.
// MonthlyChangeUSD is a signed decimal string (e.g. "-20.00") or
// "unknown" when estimation is not possible.
type DryRunBillingDelta struct {
	MonthlyChangeUSD string `json:"monthly_change_usd"`
	Note             string `json:"note,omitempty"`
}

// DryRunDetails bundles the Phase 2 enrichment a per-tool dependency walk
// produces. A walk fills whichever fields apply to its tier: Tier A fills
// Dependencies (and usually BillingDelta/Warnings), Tier B fills
// SideEffects, Tier C fills none. Empty fields stay out of the wire shape.
type DryRunDetails struct {
	Dependencies []DryRunDependency
	SideEffects  []string
	BillingDelta *DryRunBillingDelta
	Warnings     []string
}

// DryRunRequest captures the HTTP method, path, and optional sanitized body
// the mutating call would have made.
type DryRunRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
}

// IsDryRun reports whether the request's dry_run argument is the
// literal JSON boolean true. Mirrors RequireConfirm's shape so callers
// can do an early-return at the top of their handler.
func IsDryRun(request *mcp.CallToolRequest) bool {
	dryRun, ok := request.GetArguments()[paramDryRun].(bool)

	return ok && dryRun
}

// BuildDryRunResponse marshals a DryRunResponse into an MCP text
// result with the v0 wire shape. Tool handlers call this from their
// dry_run branch after fetching current_state.
//
// toolName is the registered MCP tool name (e.g.
// "linode_instance_delete"). environment is the operator-selected
// Linode environment; pass empty when the tool's caller did not
// specify one. method and path describe the HTTP call the tool would
// have made. currentState is the resource as it exists right now,
// typically fetched via the same GET endpoint the read sibling uses.
func BuildDryRunResponse(
	toolName, environment, method, path string,
	currentState any,
	body ...any,
) (*mcp.CallToolResult, error) {
	request := DryRunRequest{Method: method, Path: path}
	if len(body) > 0 {
		request.Body = body[0]
	}

	return MarshalToolResponse(DryRunResponse{
		DryRun:       true,
		Tool:         toolName,
		Environment:  environment,
		WouldExecute: request,
		CurrentState: currentState,
	})
}

// BuildDryRunResponseDetailed is the Phase 2 builder: same v0 shape plus
// the enrichment a per-tool dependency walk produced. Tier A/B handlers
// call this instead of BuildDryRunResponse after running their walk.
// Empty detail fields stay omitempty, so a walk that finds no dependencies
// produces the same wire shape as the Phase 1 builder.
func BuildDryRunResponseDetailed(
	toolName, environment, method, path string,
	currentState any,
	details *DryRunDetails,
) (*mcp.CallToolResult, error) {
	var detail DryRunDetails
	if details != nil {
		detail = *details
	}

	return MarshalToolResponse(DryRunResponse{
		DryRun:       true,
		Tool:         toolName,
		Environment:  environment,
		WouldExecute: DryRunRequest{Method: method, Path: path},
		CurrentState: currentState,
		Dependencies: detail.Dependencies,
		SideEffects:  detail.SideEffects,
		BillingDelta: detail.BillingDelta,
		Warnings:     detail.Warnings,
	})
}

// RunDryRunPreview is the shared dry-run branch for non-destroy mutating
// tools (CapWrite / CapAdmin). The caller validates required args first,
// then delegates here. When fetchState is non-nil it prepares the client
// and fetches current_state via the read sibling's GET (update tools);
// when nil, no client call is made and current_state is null (create
// tools, which have no existing resource to preview). Either way it
// emits the v0 preview and never mutates.
func RunDryRunPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	fetchState func(ctx context.Context, client *linode.Client) (any, error),
) (*mcp.CallToolResult, error) {
	return RunDryRunPreviewDetailed(ctx, request, cfg, toolName, method, path, fetchState, nil)
}

// RunDryRunPreviewDetailed is RunDryRunPreview with a Phase 2 enrichment hook.
// When detailsFn is non-nil it runs after the state fetch and returns the
// side_effects / warnings / billing_delta to attach to the preview; the
// client it receives is the same one used for the fetch (nil when fetchState
// is nil, e.g. create previews that describe the new resource from the
// request alone). A detailsFn error fails the preview, matching the destroy
// helpers' dependency-walk behavior.
func RunDryRunPreviewDetailed(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	fetchState func(ctx context.Context, client *linode.Client) (any, error),
	detailsFn func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error),
) (*mcp.CallToolResult, error) {
	var (
		state  any
		client *linode.Client
	)

	if fetchState != nil {
		preparedClient, err := prepareClient(request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client = preparedClient

		state, err = fetchState(ctx, client)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch state for dry-run: %v", err)), nil
		}
	}

	env := request.GetString(paramEnvironment, "")

	if detailsFn == nil {
		return BuildDryRunResponse(toolName, env, method, path, state)
	}

	details, err := detailsFn(ctx, client, state)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to compute dry-run side effects: %v", err)), nil
	}

	return BuildDryRunResponseDetailed(toolName, env, method, path, state, &details)
}

// RunDryRunPreviewWithBody is the shared dry-run branch for write tools whose
// safety preview needs to include the sanitized request body.
func RunDryRunPreviewWithBody(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	body any,
	fetchState func(ctx context.Context, client *linode.Client) (any, error),
) (*mcp.CallToolResult, error) {
	var state any

	if fetchState != nil {
		client, err := prepareClient(request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		state, err = fetchState(ctx, client)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch state for dry-run: %v", err)), nil
		}
	}

	return BuildDryRunResponse(toolName, request.GetString(paramEnvironment, ""), method, path, state, body)
}
