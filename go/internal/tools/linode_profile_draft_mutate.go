package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

const (
	// argTools is the JSON property name shared by _add_tools and
	// _remove_tools. Both accept an array of literal-or-wildcard
	// patterns.
	argTools = "tools"
	// argAllowedEnvironments, argRequiredTokenScopes, and argAllowYolo
	// are the _draft_set property names. Hoisted so the schema and
	// handler agree without stringly-typed drift.
	argAllowedEnvironments = "allowed_environments"
	argRequiredTokenScopes = "required_token_scopes"
	argAllowYolo           = "allow_yolo"
)

// ProfileDraftAddToolsAnswer answers linode_profile_draft_add_tools. Patterns
// expand against the live tool catalog at call time, so a wildcard like
// `linode_instance_*` picks up tools the server registered after the draft was
// created.
//
// Refusals, each answered as a tool result:
//
//   - msgDraftNameMissing when name is empty
//   - "draft not found" when the draft is not in the registry
func ProfileDraftAddToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, _ *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError(msgDraftNameMissing), nil
	}

	patterns := stringArrayArg(request, argTools)

	// AddTools answers a draft the registry does not hold and nothing else, so
	// reading the happy path first words the one refusal it can report without
	// a branch no call can reach.
	added, err := state.Drafts.AddTools(name, patterns, state.Catalog())
	if err == nil {
		return MarshalProtoToolResponse(&linodev1.ProfileDraftAddToolsResponse{
			Name:  name,
			Added: added,
		})
	}

	return mcp.NewToolResultError(draftNotFound(name)), nil
}

// ProfileDraftRemoveToolsAnswer answers linode_profile_draft_remove_tools.
// Patterns are matched against the draft's CURRENT AllowedTools, not the live
// catalog, so removing `linode_instance_*` strips exactly the instance tools
// the draft already has.
func ProfileDraftRemoveToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, _ *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError(msgDraftNameMissing), nil
	}

	patterns := stringArrayArg(request, argTools)

	// RemoveTools reports the same single failure AddTools does.
	removed, err := state.Drafts.RemoveTools(name, patterns)
	if err == nil {
		return MarshalProtoToolResponse(&linodev1.ProfileDraftRemoveToolsResponse{
			Name:    name,
			Removed: removed,
		})
	}

	return mcp.NewToolResultError(draftNotFound(name)), nil
}

// ProfileDraftSetAnswer answers linode_profile_draft_set. Each settable field
// is optional; missing fields are left unchanged. The answer reports which
// fields actually changed so the model can summarize.
func ProfileDraftSetAnswer(
	ctx context.Context, request *mcp.CallToolRequest, _ *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError(msgDraftNameMissing), nil
	}

	changes := make(map[string]*structpb.Value)

	if !applyDraftSettings(state.Drafts, name, request, changes) {
		return mcp.NewToolResultError(draftNotFound(name)), nil
	}

	return MarshalProtoToolResponse(&linodev1.ProfileDraftSetResponse{
		Name:    name,
		Changes: changes,
	})
}

// applyDraftSettings applies every settable field the call named, recording
// what changed, and reports whether the draft was there to apply them to.
//
// It answers a boolean rather than an error because a draft the registry does
// not hold is the only failure its three setters report, and the caller words
// that one refusal itself.
//
// The changes are built as response values directly rather than as a plain map
// converted afterwards: every value here is a string list or a bool, so a
// conversion step could only report a failure that cannot happen.
func applyDraftSettings(
	drafts *builder.Registry, name string, request *mcp.CallToolRequest,
	changes map[string]*structpb.Value,
) bool {
	args := request.GetArguments()

	if _, present := args[argAllowedEnvironments]; present {
		envs := stringArrayArg(request, argAllowedEnvironments)
		if drafts.SetAllowedEnvironments(name, envs) != nil {
			return false
		}

		changes[argAllowedEnvironments] = stringListChange(envs)
	}

	if _, present := args[argRequiredTokenScopes]; present {
		scopes := stringArrayArg(request, argRequiredTokenScopes)
		if drafts.SetRequiredTokenScopes(name, scopes) != nil {
			return false
		}

		changes[argRequiredTokenScopes] = stringListChange(scopes)
	}

	if _, present := args[argAllowYolo]; present {
		yolo := request.GetBool(argAllowYolo, false)
		if drafts.SetAllowYolo(name, yolo) != nil {
			return false
		}

		changes[argAllowYolo] = structpb.NewBoolValue(yolo)
	}

	return true
}

// stringListChange turns a string slice into the list value the draft-set
// response carries. structpb's constructors for the types used here cannot
// fail, which is what keeps the answer free of an unreachable error path.
func stringListChange(values []string) *structpb.Value {
	out := make([]*structpb.Value, len(values))
	for i, v := range values {
		out[i] = structpb.NewStringValue(v)
	}

	return structpb.NewListValue(&structpb.ListValue{Values: out})
}

// stringArrayArg pulls a string-array argument out of the request,
// returning an empty slice when missing or malformed. The MCP wire
// hands arrays through as []any, so we convert per-element.
//
// The request is passed by pointer rather than by value because
// mcp.CallToolRequest is ~80 bytes; gocritic flags the value copy
// for handlers that call this helper repeatedly.
func stringArrayArg(request *mcp.CallToolRequest, key string) []string {
	args := request.GetArguments()

	raw, present := args[key]
	if !present {
		return nil
	}

	asArray, isArray := raw.([]any)
	if !isArray {
		return nil
	}

	out := make([]string, 0, len(asArray))

	for _, entry := range asArray {
		text, isString := entry.(string)
		if !isString {
			continue
		}

		out = append(out, text)
	}

	return out
}
