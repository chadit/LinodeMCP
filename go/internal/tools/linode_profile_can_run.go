package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// ActiveProfileProvider returns the profile the server is currently running
// under. Injected (rather than read from a global) so linode_profile_can_run
// reflects hot-reload profile swaps at call time and tests can supply a
// reproducible fixture without standing up a Server.
type ActiveProfileProvider func() profiles.Profile

const (
	// canRunParamCalls is the input property: an array of {tool, args?}
	// entries the model intends to run as a sequence.
	canRunParamCalls = "calls"
	// canRunEntryTool / canRunEntryArgs are the per-entry object keys.
	canRunEntryTool = "tool"
	canRunEntryArgs = "args"

	// Pre-check reason strings. These are an exact-match contract shared
	// with the spec and the Python implementation; the summary bucketing
	// keys off them, so they must not drift.
	canRunReasonUnregistered = "tool name not registered"
	canRunReasonProfileBlock = "tool not in profile's allowed_tools"
	canRunReasonEnvBlock     = "environment not permitted by profile"

	// Summary bucket keys. blocked_by_reason splits profile_block and
	// capability_block (same reason string, with/without the capability
	// annotation) so analysts can count yolo-unblockable entries without
	// walking the results array.
	canRunBucketUnregistered = "unregistered"
	canRunBucketProfileBlock = "profile_block"
	canRunBucketEnvBlock     = "environment_block"
	canRunBucketCapability   = "capability_block"

	// envWildcard, when it is the sole AllowedEnvironments entry, permits
	// every configured environment (mirrors Profile.AllowedEnvironments
	// semantics: empty or ["*"] means unrestricted).
	envWildcard = "*"
)

// canRunCallResult is the per-call verdict row in the response.
type canRunCallResult struct {
	Tool    string `json:"tool"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Remedy  string `json:"remedy,omitempty"`
}

// canRunSummary collapses the per-call verdicts into counts. BlockedByReason
// holds the four bucket keys; its sum is <= Blocked (a future uncategorized
// reason would increment Blocked without a bucket).
type canRunSummary struct {
	Total           int            `json:"total"`
	Allowed         int            `json:"allowed"`
	Blocked         int            `json:"blocked"`
	BlockedByReason map[string]int `json:"blocked_by_reason"`
}

// canRunResponse is the full wire shape of linode_profile_can_run.
type canRunResponse struct {
	ActiveProfile string             `json:"active_profile"`
	Results       []canRunCallResult `json:"results"`
	Summary       canRunSummary      `json:"summary"`
}

// profilePermitsAllEnvironments reports whether the profile imposes no
// environment restriction: an empty list, or a list whose only entry is "*".
func profilePermitsAllEnvironments(envs []string) bool {
	return len(envs) == 0 || (len(envs) == 1 && envs[0] == envWildcard)
}

// NewLinodeProfileCanRunTool returns the linode_profile_can_run pre-check
// tool. It answers "would the active profile permit this sequence of tool
// calls?" so the model can bail before partial execution strands the user.
// It inspects only the tool name and the optional `environment` arg of each
// call against the active profile; it does not check resource IDs, API token
// scope, resource existence, or rate limits. Pre-check is advice, not a plan.
//
// Both providers run at handler call time so hot-reload changes to the
// catalog or active profile are reflected without re-registering the tool.
func NewLinodeProfileCanRunTool(
	catalog CatalogProvider,
	activeProfile ActiveProfileProvider,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_can_run",
		mcp.WithDescription(
			"Pre-check whether the active profile would permit a sequence of tool "+
				"calls before executing any of them. Returns a per-call allowed/blocked "+
				"verdict with a reason and remedy, plus a summary. Inspects only the tool "+
				"name and optional environment arg, not resource IDs. Advice only; it does "+
				"not execute anything.",
		),
		mcp.WithArray(
			canRunParamCalls,
			mcp.Description("Tool calls to pre-check. Each entry is an object with a required "+
				"\"tool\" name and optional \"args\" (only \"environment\" is inspected)."),
			mcp.Required(),
			mcp.Items(canRunCallItemSchema()),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		profile := activeProfile()
		registered := registeredCapabilities(catalog())
		allowedTools := sliceToSet(profile.AllowedTools)
		allEnvs := profilePermitsAllEnvironments(profile.AllowedEnvironments)

		rawCalls, _ := request.GetArguments()[canRunParamCalls].([]any)

		resp := canRunResponse{
			ActiveProfile: profile.Name,
			Results:       make([]canRunCallResult, 0, len(rawCalls)),
			Summary: canRunSummary{
				BlockedByReason: map[string]int{
					canRunBucketUnregistered: 0,
					canRunBucketProfileBlock: 0,
					canRunBucketEnvBlock:     0,
					canRunBucketCapability:   0,
				},
			},
		}

		for _, raw := range rawCalls {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}

			toolName, _ := entry[canRunEntryTool].(string)
			env, hasEnv := canRunEntryEnvironment(entry)

			result, bucket := evaluateCanRun(toolName, env, hasEnv, registered, allowedTools, profile.AllowedEnvironments, allEnvs)
			resp.Results = append(resp.Results, result)
			resp.Summary.Total++

			if result.Allowed {
				resp.Summary.Allowed++

				continue
			}

			resp.Summary.Blocked++
			resp.Summary.BlockedByReason[bucket]++
		}

		body, err := json.Marshal(resp)
		if err != nil {
			return nil, fmt.Errorf("marshal can_run response: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// canRunCallItemSchema is the mcp.Items schema for one `calls` entry: an
// object with a required string `tool` and an optional `args` object.
// Returns a fresh map per call so the global-state linter has nothing to flag.
func canRunCallItemSchema() map[string]any {
	return map[string]any{
		jsonSchemaTypeKey: "object",
		"properties": map[string]any{
			canRunEntryTool: map[string]any{jsonSchemaTypeKey: jsonSchemaStringType},
			canRunEntryArgs: map[string]any{jsonSchemaTypeKey: "object"},
		},
		"required": []any{canRunEntryTool},
	}
}

// canRunEntryEnvironment extracts the optional environment arg from a call
// entry's args object. Returns ("", false) when args or environment is absent
// or not a non-empty string.
func canRunEntryEnvironment(entry map[string]any) (string, bool) {
	rawArgs, hasArgs := entry[canRunEntryArgs].(map[string]any)
	if !hasArgs {
		return "", false
	}

	env, isString := rawArgs[paramEnvironment].(string)
	if !isString || env == "" {
		return "", false
	}

	return env, true
}

// registeredCapabilities maps every registered tool name to its capability so
// the pre-check can both detect unregistered names and annotate destructive
// blocks.
func registeredCapabilities(catalog []profiles.ToolDescriptor) map[string]profiles.Capability {
	out := make(map[string]profiles.Capability, len(catalog))
	for idx := range catalog {
		out[catalog[idx].Name] = catalog[idx].Capability
	}

	return out
}

// sliceToSet builds a set from a string slice for O(1) membership checks.
func sliceToSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}

	return out
}

// evaluateCanRun resolves a single call to its verdict and summary bucket.
// The bucket is the empty string when the call is allowed. Refusal order
// mirrors real dispatch: unregistered first, then profile membership, then
// environment.
func evaluateCanRun(
	toolName, env string,
	hasEnv bool,
	registered map[string]profiles.Capability,
	allowedTools map[string]struct{},
	allowedEnvs []string,
	allEnvs bool,
) (canRunCallResult, string) {
	result := canRunCallResult{Tool: toolName}

	capability, isRegistered := registered[toolName]
	if !isRegistered {
		result.Reason = canRunReasonUnregistered
		result.Remedy = "check spelling or call linode_profile_list_tools to discover the registered tool surface"

		return result, canRunBucketUnregistered
	}

	if _, permitted := allowedTools[toolName]; !permitted {
		if capability == profiles.CapDestroy {
			result.Reason = canRunReasonProfileBlock + " (" + capability.String() + ")"
			result.Remedy = "switch to a profile that permits " + toolName + ", or use yolo on a profile that allows it"

			return result, canRunBucketCapability
		}

		result.Reason = canRunReasonProfileBlock
		result.Remedy = "switch to a profile that permits " + toolName + ", or add it to the current profile"

		return result, canRunBucketProfileBlock
	}

	if hasEnv && !allEnvs && !slices.Contains(allowedEnvs, env) {
		result.Reason = canRunReasonEnvBlock
		result.Remedy = "target an environment in the profile's allowed_environments, or switch to a profile that permits this environment"

		return result, canRunBucketEnvBlock
	}

	result.Allowed = true

	return result, ""
}
