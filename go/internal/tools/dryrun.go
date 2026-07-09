package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

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
	msg, err := buildDryRunProto(toolName, environment, method, path, currentState, nil, body...)
	if err != nil {
		return nil, err
	}

	return MarshalProtoToolResponse(msg)
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
	body ...any,
) (*mcp.CallToolResult, error) {
	msg, err := buildDryRunProto(toolName, environment, method, path, currentState, details, body...)
	if err != nil {
		return nil, err
	}

	return MarshalProtoToolResponse(msg)
}

// buildDryRunProto assembles the DryRunResponse proto from the builder inputs.
// currentState is always emitted (JSON null when nil, so a create preview still
// reports the would-be request); the sanitized body stays absent unless a
// non-nil one is supplied. When details is non-nil its dependency-walk fields
// are attached. current_state and body serialize through structpb.Value so
// protojson sorts their object keys on both languages.
func buildDryRunProto(
	toolName, environment, method, path string,
	currentState any,
	details *DryRunDetails,
	body ...any,
) (*linodev1.DryRunResponse, error) {
	request, err := buildWouldExecute(method, path, body...)
	if err != nil {
		return nil, err
	}

	stateValue, err := toProtoValue(currentState)
	if err != nil {
		return nil, err
	}

	msg := &linodev1.DryRunResponse{
		DryRun:       true,
		Tool:         toolName,
		WouldExecute: request,
		CurrentState: stateValue,
	}

	if environment != "" {
		msg.Environment = new(environment)
	}

	if details != nil {
		deps, depErr := dependenciesToProto(details.Dependencies)
		if depErr != nil {
			return nil, depErr
		}

		msg.Dependencies = deps
		msg.SideEffects = details.SideEffects
		msg.BillingDelta = billingDeltaToProto(details.BillingDelta)
		msg.Warnings = details.Warnings
	}

	return msg, nil
}

// buildWouldExecute builds the would_execute preview. The sanitized body is
// attached only when a non-nil one is supplied, matching the old omitempty tag.
func buildWouldExecute(method, path string, body ...any) (*linodev1.DryRunRequest, error) {
	request := &linodev1.DryRunRequest{Method: method, Path: path}

	if len(body) > 0 && body[0] != nil {
		bodyValue, err := toProtoValue(body[0])
		if err != nil {
			return nil, err
		}

		request.Body = bodyValue
	}

	return request, nil
}

// dependenciesToProto converts the walk's Go dependencies into their proto
// form. An empty slice yields nil so the proto emits an empty array only when
// the marshaller's EmitDefaultValues fills it, matching the other repeated
// fields. Optional label/note stay absent when empty.
func dependenciesToProto(deps []DryRunDependency) ([]*linodev1.DryRunDependency, error) {
	if len(deps) == 0 {
		return nil, nil
	}

	out := make([]*linodev1.DryRunDependency, 0, len(deps))

	for idx := range deps {
		source := deps[idx]
		dep := &linodev1.DryRunDependency{Kind: source.Kind, Action: source.Action}

		if source.ID != nil {
			idValue, err := toProtoValue(source.ID)
			if err != nil {
				return nil, err
			}

			dep.Id = idValue
		}

		if source.Label != "" {
			dep.Label = new(source.Label)
		}

		if source.Note != "" {
			dep.Note = new(source.Note)
		}

		out = append(out, dep)
	}

	return out, nil
}

// billingDeltaToProto converts the optional billing estimate; nil stays nil so
// the proto field is omitted.
func billingDeltaToProto(delta *DryRunBillingDelta) *linodev1.DryRunBillingDelta {
	if delta == nil {
		return nil
	}

	out := &linodev1.DryRunBillingDelta{MonthlyChangeUsd: delta.MonthlyChangeUSD}
	if delta.Note != "" {
		out.Note = new(delta.Note)
	}

	return out
}

// toProtoValue converts an arbitrary current_state or body value into a
// structpb.Value. A nil value becomes an explicit JSON null. Everything else
// round-trips through JSON first so typed read-sibling models (the shape the
// dependency walks fetch) collapse to the plain map/slice/scalar form
// structpb.NewValue accepts, keeping their JSON field names.
func toProtoValue(value any) (*structpb.Value, error) {
	if value == nil {
		return structpb.NewNullValue(), nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal value for proto: %w", err)
	}

	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil, fmt.Errorf("unmarshal value for proto: %w", err)
	}

	protoValue, err := structpb.NewValue(generic)
	if err != nil {
		return nil, fmt.Errorf("build proto value: %w", err)
	}

	return protoValue, nil
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

// RunDryRunPreviewWithBodyDetailed is the Phase 2 variant of
// RunDryRunPreviewWithBody: it threads an optional per-tool side-effect walk
// (detailsFn) that runs after the state fetch, so body-carrying write tools
// can surface side_effects/warnings alongside the sanitized request body. A
// nil detailsFn reproduces RunDryRunPreviewWithBody's behavior.
func RunDryRunPreviewWithBodyDetailed(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	body any,
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
		return BuildDryRunResponse(toolName, env, method, path, state, body)
	}

	details, err := detailsFn(ctx, client, state)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to compute dry-run side effects: %v", err)), nil
	}

	return BuildDryRunResponseDetailed(toolName, env, method, path, state, &details, body)
}
