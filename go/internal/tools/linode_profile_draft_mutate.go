package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
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
	// respFieldName is the response JSON key carrying the draft name.
	// All three mutator handlers echo the name back so the model can
	// pair responses with the request that produced them.
	respFieldName = "name"
	// jsonSchemaTypeKey and jsonSchemaStringType form the
	// {"type": "string"} item schema used in mcp.Items. Hoisting both
	// the key and value avoids tripping the goconst linter as the
	// repetition grows across the three mutator schemas.
	jsonSchemaTypeKey    = "type"
	jsonSchemaStringType = "string"
)

// schemaStringItem returns the mcp.Items schema for a string-typed
// array element. Returns a fresh map per call so callers cannot
// share-mutate, and the global-state linter has nothing to flag.
func schemaStringItem() map[string]any {
	return map[string]any{jsonSchemaTypeKey: jsonSchemaStringType}
}

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
	tool := mcp.NewTool(
		"linode_profile_draft_add_tools",
		mcp.WithDescription(
			"Add tools to a profile draft. Accepts literal tool names and "+
				"wildcards (shell-glob, only '*' is special). Wildcards "+
				"expand against the live tool catalog at call time. Names "+
				"already on the draft are not duplicated and are not "+
				"reported in the response.",
		),
		mcp.WithString(
			"name",
			mcp.Description("Draft name to mutate."),
			mcp.Required(),
		),
		mcp.WithArray(
			argTools,
			mcp.Description("List of tool names or wildcard patterns to add."),
			mcp.Required(),
			mcp.Items(schemaStringItem()),
		),
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

		body, err := json.Marshal(map[string]any{respFieldName: name, "added": added})
		if err != nil {
			return nil, fmt.Errorf("marshal add result: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
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
	tool := mcp.NewTool(
		"linode_profile_draft_remove_tools",
		mcp.WithDescription(
			"Remove tools from a profile draft. Accepts literal tool names "+
				"and wildcards (shell-glob, only '*' is special). Patterns "+
				"match against the draft's current AllowedTools list, not "+
				"the live catalog.",
		),
		mcp.WithString(
			"name",
			mcp.Description("Draft name to mutate."),
			mcp.Required(),
		),
		mcp.WithArray(
			argTools,
			mcp.Description("List of tool names or wildcard patterns to remove."),
			mcp.Required(),
			mcp.Items(schemaStringItem()),
		),
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

		body, err := json.Marshal(map[string]any{respFieldName: name, "removed": removed})
		if err != nil {
			return nil, fmt.Errorf("marshal remove result: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
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
	tool := mcp.NewTool(
		"linode_profile_draft_set",
		mcp.WithDescription(
			"Set draft settings. Each field is optional; missing fields are "+
				"left unchanged. Settable: allowed_environments (array of "+
				"environment names), required_token_scopes (array of Linode "+
				"scope strings), allow_yolo (boolean opt-in to the yolo "+
				"execution path).",
		),
		mcp.WithString(
			"name",
			mcp.Description("Draft name to mutate."),
			mcp.Required(),
		),
		mcp.WithArray(
			argAllowedEnvironments,
			mcp.Description("Replace allowed_environments with this list."),
			mcp.Items(schemaStringItem()),
		),
		mcp.WithArray(
			argRequiredTokenScopes,
			mcp.Description("Replace required_token_scopes with this list."),
			mcp.Items(schemaStringItem()),
		),
		mcp.WithBoolean(
			argAllowYolo,
			mcp.Description("Set the allow_yolo flag."),
		),
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

			changes[argAllowedEnvironments] = envs
		}

		if _, present := args[argRequiredTokenScopes]; present {
			scopes := stringArrayArg(&request, argRequiredTokenScopes)
			if err := registry.SetRequiredTokenScopes(name, scopes); err != nil {
				return nil, fmt.Errorf("set required_token_scopes on draft %q: %w", name, err)
			}

			changes[argRequiredTokenScopes] = scopes
		}

		if _, present := args[argAllowYolo]; present {
			yolo := request.GetBool(argAllowYolo, false)
			if err := registry.SetAllowYolo(name, yolo); err != nil {
				return nil, fmt.Errorf("set allow_yolo on draft %q: %w", name, err)
			}

			changes[argAllowYolo] = yolo
		}

		body, err := json.Marshal(map[string]any{respFieldName: name, "changes": changes})
		if err != nil {
			return nil, fmt.Errorf("marshal set result: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
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
