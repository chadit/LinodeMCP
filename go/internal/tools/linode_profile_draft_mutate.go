package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/structpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// CatalogSnapshot returns the full tool catalog. The Phase 8.4
// `_draft_add_tools` handler uses this to expand wildcards. Provided
// as a function type so tests can supply reproducible fixtures
// without spinning up a Server.
type CatalogSnapshot func() []profiles.ToolDescriptor

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

// NewLinodeProfileDraftAddToolsTool returns the
// linode_profile_draft_add_tools builder tool. Patterns expand
// against the live tool catalog at call time, so a wildcard like
// `linode_instance_*` picks up tools the server registered after
// the draft was created.
//
// Errors at handler call time:
//
//   - ErrDraftNameMissing when name is empty
//   - builder.ErrDraftNotFound when the draft is not in the registry
func NewLinodeProfileDraftAddToolsTool(
	registry *builder.Registry,
	catalog CatalogSnapshot,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_add_tools",
		"Add tools to a profile draft. Accepts literal tool names and "+
			"wildcards (shell-glob, only '*' is special). Wildcards "+
			"expand against the live tool catalog at call time. Names "+
			"already on the draft are not duplicated and are not "+
			"reported in the response.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftAddToolsInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := request.GetString("name", "")
		if name == "" {
			return nil, ErrDraftNameMissing
		}

		patterns := stringArrayArg(&request, argTools)

		added, err := registry.AddTools(name, patterns, catalog())
		if err != nil {
			return nil, fmt.Errorf("add tools to draft %q: %w", name, err)
		}

		return MarshalProtoToolResponse(&linodev1.ProfileDraftAddToolsResponse{
			Name:  name,
			Added: added,
		})
	}

	return tool, profiles.CapMeta, handler
}

// NewLinodeProfileDraftRemoveToolsTool returns the
// linode_profile_draft_remove_tools builder tool. Patterns are
// matched against the draft's CURRENT AllowedTools, not the live
// catalog, so removing `linode_instance_*` strips exactly the
// instance tools the draft already has.
func NewLinodeProfileDraftRemoveToolsTool(
	registry *builder.Registry,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_remove_tools",
		"Remove tools from a profile draft. Accepts literal tool names "+
			"and wildcards (shell-glob, only '*' is special). Patterns "+
			"match against the draft's current AllowedTools list, not "+
			"the live catalog.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftRemoveToolsInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := request.GetString("name", "")
		if name == "" {
			return nil, ErrDraftNameMissing
		}

		patterns := stringArrayArg(&request, argTools)

		removed, err := registry.RemoveTools(name, patterns)
		if err != nil {
			return nil, fmt.Errorf("remove tools from draft %q: %w", name, err)
		}

		return MarshalProtoToolResponse(&linodev1.ProfileDraftRemoveToolsResponse{
			Name:    name,
			Removed: removed,
		})
	}

	return tool, profiles.CapMeta, handler
}

// NewLinodeProfileDraftSetTool returns the linode_profile_draft_set
// builder tool. Each settable field is optional; missing fields are
// left unchanged. The response reports which fields actually changed
// so the model can summarize.
func NewLinodeProfileDraftSetTool(
	registry *builder.Registry,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_set",
		"Set draft settings. Each field is optional; missing fields are "+
			"left unchanged. Settable: allowed_environments (array of "+
			"environment names), required_token_scopes (array of Linode "+
			"scope strings), allow_yolo (boolean opt-in to the yolo "+
			"execution path).",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftSetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := request.GetString("name", "")
		if name == "" {
			return nil, ErrDraftNameMissing
		}

		changes := make(map[string]any)
		args := request.GetArguments()

		if _, present := args[argAllowedEnvironments]; present {
			envs := stringArrayArg(&request, argAllowedEnvironments)
			if err := registry.SetAllowedEnvironments(name, envs); err != nil {
				return nil, fmt.Errorf("set allowed_environments on draft %q: %w", name, err)
			}

			changes[argAllowedEnvironments] = stringsToAnySlice(envs)
		}

		if _, present := args[argRequiredTokenScopes]; present {
			scopes := stringArrayArg(&request, argRequiredTokenScopes)
			if err := registry.SetRequiredTokenScopes(name, scopes); err != nil {
				return nil, fmt.Errorf("set required_token_scopes on draft %q: %w", name, err)
			}

			changes[argRequiredTokenScopes] = stringsToAnySlice(scopes)
		}

		if _, present := args[argAllowYolo]; present {
			yolo := request.GetBool(argAllowYolo, false)
			if err := registry.SetAllowYolo(name, yolo); err != nil {
				return nil, fmt.Errorf("set allow_yolo on draft %q: %w", name, err)
			}

			changes[argAllowYolo] = yolo
		}

		changesStruct, err := structpb.NewStruct(changes)
		if err != nil {
			return nil, fmt.Errorf("convert set changes: %w", err)
		}

		return MarshalProtoToolResponse(&linodev1.ProfileDraftSetResponse{
			Name:    name,
			Changes: changesStruct.GetFields(),
		})
	}

	return tool, profiles.CapMeta, handler
}

// stringsToAnySlice widens a string slice to []any so it can pass through
// structpb.NewStruct, which accepts only JSON-native value types.
func stringsToAnySlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}

	return out
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
