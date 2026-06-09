package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

// Shared literals for destroy-tool response building. Extracted so
// per-tool struct literals don't each repeat them (which trips
// goconst once enough tools accumulate the same string).
const (
	httpMethodDelete   = "DELETE"
	httpMethodPost     = "POST"
	httpMethodPut      = "PUT"
	responseKeyMessage = "message"

	// destroyConfirmMessage is the generic confirm gate for destroy tools
	// whose resource type is already clear from the tool name.
	destroyConfirmMessage = "This operation is destructive. Set confirm=true to proceed."
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

	// DependencyWalk, when non-nil, runs the Phase 2 dependency walk on a
	// dry-run after FetchState succeeds, enriching the preview with
	// dependencies, side-effects, billing delta, and warnings. Nil keeps
	// the Phase 1 behavior (preview carries would_execute + current_state
	// only). The walk receives the already-fetched state so it can avoid a
	// redundant GET.
	DependencyWalk func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error)

	// HashIgnore lists the cosmetic state fields stripped before the two-stage
	// drift hash, so a plan does not refuse on a field that moves without a
	// user-caused change. Nil hashes the whole state. Use
	// twostage.HashIgnoreFields(resourceType) to populate it.
	HashIgnore []string
}

// runDestructiveDryRun handles the dry-run branch of the destroy flow:
// prepare the client, fetch current state, run the optional dependency
// walk, and emit the preview. Extracted from RunDestructiveAction to keep
// that function's nesting flat.
func runDestructiveDryRun(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	state, fetchErr := action.FetchState(ctx, client)
	if fetchErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch state for dry-run: %v", fetchErr)), nil
	}

	env := request.GetString(paramEnvironment, "")

	if action.DependencyWalk == nil {
		return BuildDryRunResponse(action.ToolName, env, action.Method, action.Path, state)
	}

	details, walkErr := action.DependencyWalk(ctx, client, state)
	if walkErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to compute dry-run dependencies: %v", walkErr)), nil
	}

	return BuildDryRunResponseDetailed(action.ToolName, env, action.Method, action.Path, state, &details)
}

// RunDestructiveAction runs the shared destroy-tool flow. On dry-run:
// prepare client, fetch state, emit the v0 preview. On real execution:
// gate on confirm first (so an unconfirmed call short-circuits before
// touching the API client), then prepare client and execute. Per-tool
// validation happens in the caller.
func RunDestructiveAction(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, error) {
	if result, handled := runTwoStageBranch(ctx, request, cfg, action); handled {
		return result, nil
	}

	if IsDryRun(request) {
		return runDestructiveDryRun(ctx, request, cfg, action)
	}

	if result := requireDestroyConfirmation(ctx, request, action.ToolName, action.ConfirmMessage); result != nil {
		return result, nil
	}

	return executeDestroy(ctx, request, cfg, action)
}

// destroyCtxKey namespaces context values this package reads. The server
// middleware sets yoloAllowedCtxKey when a call passes yolo:true and the
// active profile permits yolo, so the destroy gate can honor it without a
// package global or a change to every caller's signature.
type destroyCtxKey int

const (
	yoloAllowedCtxKey destroyCtxKey = iota
	planStoreCtxKey
)

// WithYoloAllowed marks a context as permitted to bypass the destroy gate and
// confirm requirement via yolo. The server middleware sets this only when the
// request asked for yolo and the active profile's allow_yolo is true.
func WithYoloAllowed(ctx context.Context) context.Context {
	return context.WithValue(ctx, yoloAllowedCtxKey, true)
}

// yoloAllowedFromContext reports whether the server marked this call as a
// permitted yolo execution.
func yoloAllowedFromContext(ctx context.Context) bool {
	allowed, _ := ctx.Value(yoloAllowedCtxKey).(bool)

	return allowed
}

// WithPlanStore attaches the server's two-stage plan store to a context so a
// destroy handler can produce and consume plans. The server middleware sets it
// on every call. Handlers that are not two-stage-aware ignore it.
func WithPlanStore(ctx context.Context, store *twostage.PlanStore) context.Context {
	return context.WithValue(ctx, planStoreCtxKey, store)
}

// PlanStoreFromContext returns the plan store the server attached, or nil when
// two-stage is not wired (for example in unit tests that call a handler
// directly). A nil store means the handler uses its single-step path.
func PlanStoreFromContext(ctx context.Context) *twostage.PlanStore {
	store, _ := ctx.Value(planStoreCtxKey).(*twostage.PlanStore)

	return store
}

// requireDestroyConfirmation enforces the Phase 3 bypass-dry-run gate for
// CapDestroy tools. Beyond the existing confirm:true requirement, a real
// destructive call must assert one of:
//
//   - confirmed_dry_run: true  -- the model previewed the call with dry_run
//   - confirm_bypass_dry_run: true -- the model is explicitly skipping the preview
//
// A permitted yolo execution (yolo:true on a profile with allow_yolo) bypasses
// the gate and the confirm requirement entirely.
// Returns a non-nil error result to short-circuit, or nil to proceed.
func requireDestroyConfirmation(ctx context.Context, request *mcp.CallToolRequest, toolName, confirmMessage string) *mcp.CallToolResult {
	if yoloAllowedFromContext(ctx) {
		return nil
	}

	args := request.GetArguments()
	confirm, _ := args[paramConfirm].(bool)
	confirmedDryRun, _ := args[paramConfirmedDryRun].(bool)
	bypass, _ := args[paramConfirmBypassDryRun].(bool)

	if bypass && confirmedDryRun {
		return mcp.NewToolResultError(
			"Pass either confirm_bypass_dry_run (skip preview) or confirmed_dry_run (preview was done), not both",
		)
	}

	if !confirm {
		if bypass {
			return mcp.NewToolResultError("confirm_bypass_dry_run only takes effect with confirm: true")
		}

		return mcp.NewToolResultError(confirmMessage)
	}

	if !confirmedDryRun && !bypass {
		return mcp.NewToolResultError(destroyBypassMessage(toolName))
	}

	return nil
}

// destroyBypassMessage is the error returned when a CapDestroy tool is called
// with confirm:true but without a prior dry-run assertion or an explicit
// bypass. It tells the model the three ways forward.
func destroyBypassMessage(toolName string) string {
	return fmt.Sprintf(
		"%s is destructive. Either:\n"+
			"  1. Call with dry_run: true first to preview, then call again with\n"+
			"     confirm: true, confirmed_dry_run: true\n"+
			"  2. Call with confirm: true, confirm_bypass_dry_run: true to skip preview\n"+
			"  3. Use yolo: true (only if profile allows)",
		toolName,
	)
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

	// DependencyWalk, when non-nil, runs the Phase 2 dependency walk on a
	// dry-run after FetchState. It receives the parsed ID and fetched state.
	// Nil keeps the Phase 1 preview (would_execute + current_state only).
	DependencyWalk func(ctx context.Context, client *linode.Client, id int, state any) (DryRunDetails, error)

	// HashIgnore lists cosmetic state fields stripped before the two-stage
	// drift hash. Use twostage.HashIgnoreFields(resourceType) to populate it.
	HashIgnore []string
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

	var walk func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error)
	if params.DependencyWalk != nil {
		walk = func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error) {
			return params.DependencyWalk(ctx, client, id, state)
		}
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
		DependencyWalk: walk,
		HashIgnore:     params.HashIgnore,
	})
}

// DestructiveActionByTwoIDs configures a destroy tool keyed by a
// pair of integer IDs in a parent/child path shape (e.g. `/domains/{d}/records/{r}`,
// `/vpcs/{v}/subnets/{s}`, `/networking/firewalls/{f}/devices/{d}`).
// PathPattern takes two %d slots: outer first, then inner.
// SuccessFormat is fmt.Sprintf'd with (inner, outer) in that order to
// match the legacy "Record %d removed successfully from domain %d"
// shape; a format with no %d slots (e.g. for tools whose success
// message is static) is also fine since fmt.Sprintf ignores extras.
type DestructiveActionByTwoIDs struct {
	ToolName       string
	OuterIDParam   string
	InnerIDParam   string
	Method         string
	PathPattern    string
	ConfirmMessage string
	SuccessFormat  string
	FetchState     func(ctx context.Context, client *linode.Client, outerID, innerID int) (any, error)
	Execute        func(ctx context.Context, client *linode.Client, outerID, innerID int) error

	// DependencyWalk, when non-nil, runs the Phase 2 dependency walk on a
	// dry-run after FetchState. It receives both parsed IDs and the fetched
	// state. Nil keeps the Phase 1 preview (would_execute + current_state).
	DependencyWalk func(ctx context.Context, client *linode.Client, outerID, innerID int, state any) (DryRunDetails, error)

	// HashIgnore lists cosmetic state fields stripped before the two-stage
	// drift hash. Use twostage.HashIgnoreFields(resourceType) to populate it.
	HashIgnore []string
}

// RunDestructiveActionByTwoIDs is the two-int-ID convenience wrapper
// over RunDestructiveAction. It parses and validates both ID args
// (`== 0` rejection only; tools that need stricter checks like
// `<= 0` for negatives should add a pre-validation guard in the
// handler before invoking this, matching the object_storage_key_delete
// pattern from Phase 1b.2). Per-tool handlers reduce to a single
// struct literal.
func RunDestructiveActionByTwoIDs(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	params *DestructiveActionByTwoIDs,
) (*mcp.CallToolResult, error) {
	outerID := request.GetInt(params.OuterIDParam, 0)
	if outerID == 0 {
		return mcp.NewToolResultError(params.OuterIDParam + " is required"), nil
	}

	innerID := request.GetInt(params.InnerIDParam, 0)
	if innerID == 0 {
		return mcp.NewToolResultError(params.InnerIDParam + " is required"), nil
	}

	var walk func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error)
	if params.DependencyWalk != nil {
		walk = func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error) {
			return params.DependencyWalk(ctx, client, outerID, innerID, state)
		}
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       params.ToolName,
		Method:         params.Method,
		Path:           fmt.Sprintf(params.PathPattern, outerID, innerID),
		ConfirmMessage: params.ConfirmMessage,
		FetchState: func(ctx context.Context, client *linode.Client) (any, error) {
			return params.FetchState(ctx, client, outerID, innerID)
		},
		Execute: func(ctx context.Context, client *linode.Client) error {
			return params.Execute(ctx, client, outerID, innerID)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage:  fmt.Sprintf(params.SuccessFormat, innerID, outerID),
				params.OuterIDParam: outerID,
				params.InnerIDParam: innerID,
			}
		},
		DependencyWalk: walk,
		HashIgnore:     params.HashIgnore,
	})
}

// DestructiveActionByRegionLabel configures a destroy tool keyed by
// the (region, label) pair, the canonical Object Storage path shape.
// PathPattern takes two %s slots: region first, then label. The success
// response always carries "region", plus a label value keyed by
// SuccessKey ("label" or "bucket", per legacy response shapes).
// SuccessFormat takes two %s slots: label first, then region.
type DestructiveActionByRegionLabel struct {
	ToolName       string
	Method         string
	PathPattern    string
	ConfirmMessage string
	SuccessKey     string
	SuccessFormat  string
	FetchState     func(ctx context.Context, client *linode.Client, region, label string) (any, error)
	Execute        func(ctx context.Context, client *linode.Client, region, label string) error
}

// RunDestructiveActionByRegionLabel is the (region, label) convenience
// wrapper over RunDestructiveAction. It parses and validates both args
// against the bucket sentinels, then delegates to the underlying flow
// with closures that capture region/label. Per-tool handlers reduce to
// a single struct literal, below dupl's threshold.
func RunDestructiveActionByRegionLabel(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	params *DestructiveActionByRegionLabel,
) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError(ErrBucketRegionRequired.Error()), nil
	}

	if label == "" {
		return mcp.NewToolResultError(ErrBucketLabelRequired.Error()), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       params.ToolName,
		Method:         params.Method,
		Path:           fmt.Sprintf(params.PathPattern, region, label),
		ConfirmMessage: params.ConfirmMessage,
		FetchState: func(ctx context.Context, client *linode.Client) (any, error) {
			return params.FetchState(ctx, client, region, label)
		},
		Execute: func(ctx context.Context, client *linode.Client) error {
			return params.Execute(ctx, client, region, label)
		},
		Success: func() any {
			return map[string]any{
				responseKeyMessage: fmt.Sprintf(params.SuccessFormat, label, region),
				"region":           region,
				params.SuccessKey:  label,
			}
		},
	})
}
