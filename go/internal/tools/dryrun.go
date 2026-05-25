package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
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
}

// DryRunRequest captures the HTTP method and path the mutating call
// would have made. Body is intentionally omitted in Phase 1; if a
// future phase needs to surface the request body (with sensitive
// fields redacted), it lands here alongside Method and Path.
type DryRunRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
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
) (*mcp.CallToolResult, error) {
	return MarshalToolResponse(DryRunResponse{
		DryRun:       true,
		Tool:         toolName,
		Environment:  environment,
		WouldExecute: DryRunRequest{Method: method, Path: path},
		CurrentState: currentState,
	})
}
