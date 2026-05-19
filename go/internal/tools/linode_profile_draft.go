package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/profiles/builder"
)

// ProfileResolver returns a Profile by name across both built-in and
// user-defined catalogs. The Phase 8.3 _draft_new tool uses this to
// seed a new draft from the named clone_from profile. Returning
// (zero, false) means "no such profile".
type ProfileResolver func(name string) (profiles.Profile, bool)

// draftJSONShape is the wire shape for a draft response. The JSON tags
// match the Python side so both implementations produce identical
// payloads. Fields mirror builder.Draft except Name comes first for
// readability.
type draftJSONShape struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	AllowedTools        []string `json:"allowed_tools"`
	AllowedEnvironments []string `json:"allowed_environments"`
	RequiredTokenScopes []string `json:"required_token_scopes"`
	AllowYolo           bool     `json:"allow_yolo"`
}

// draftJSON serializes a builder.Draft into the wire shape. The empty
// slice substitution (nil to []) keeps the JSON output as “[]“ not
// “null“ so the model's parser doesn't have to handle both.
func draftJSON(draft *builder.Draft) draftJSONShape {
	return draftJSONShape{
		Name:                draft.Name,
		Description:         draft.Description,
		AllowedTools:        emptyIfNil(draft.AllowedTools),
		AllowedEnvironments: emptyIfNil(draft.AllowedEnvironments),
		RequiredTokenScopes: emptyIfNil(draft.RequiredTokenScopes),
		AllowYolo:           draft.AllowYolo,
	}
}

// emptyIfNil ensures a slice marshals to “[]“ rather than “null“.
// Builder.Draft starts with nil slices when constructed without a
// clone source; the JSON contract promises arrays.
func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}

// NewLinodeProfileDraftNewTool returns the linode_profile_draft_new
// builder tool. It starts a new draft in the server's draft registry.
// Optional “clone_from“ seeds the draft from an existing profile
// (built-in or user-defined); without it the draft starts empty.
//
// Errors at handler call time:
//
//   - ErrDraftNameMissing when “name“ is empty
//   - builder.ErrDraftExists when the name is already drafted
//   - ErrCloneSourceMissing when “clone_from“ is non-empty but
//     resolves to no profile
func NewLinodeProfileDraftNewTool(
	registry *builder.Registry,
	resolver ProfileResolver,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_draft_new",
		mcp.WithDescription(
			"Start a new profile draft in the server's in-memory builder "+
				"registry. Optional clone_from seeds the draft from an "+
				"existing built-in or user-defined profile. The draft "+
				"persists only for this server's lifetime; use "+
				"linode_profile_draft_save (Phase 8.5) to write it to "+
				"the config file.",
		),
		mcp.WithString(
			"name",
			mcp.Description("Name for the new draft. Must be unique within the registry."),
			mcp.Required(),
		),
		mcp.WithString(
			"clone_from",
			mcp.Description("Optional profile name to seed the draft from. Resolves against built-ins and user-defined profiles; user-defined shadow built-ins by name."),
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

		cloneFrom := request.GetString("clone_from", "")

		var source *profiles.Profile

		if cloneFrom != "" {
			resolved, ok := resolver(cloneFrom)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrCloneSourceMissing, cloneFrom)
			}

			source = &resolved
		}

		draft, err := registry.Create(name, source)
		if err != nil {
			return nil, fmt.Errorf("create draft %q: %w", name, err)
		}

		body, err := json.Marshal(draftJSON(draft))
		if err != nil {
			return nil, fmt.Errorf("marshal draft: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// NewLinodeProfileDraftShowTool returns the linode_profile_draft_show
// builder tool. It reads the named draft and returns its current
// state. A miss returns an error so the model can surface the typo or
// expired session.
func NewLinodeProfileDraftShowTool(
	registry *builder.Registry,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_draft_show",
		mcp.WithDescription(
			"Show the current state of a profile draft. Returns name, "+
				"description, allowed tools, allowed environments, "+
				"required token scopes, and the allow_yolo flag.",
		),
		mcp.WithString(
			"name",
			mcp.Description("Draft name to show."),
			mcp.Required(),
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

		draft, ok := registry.Get(name)
		if !ok {
			return nil, fmt.Errorf("draft %q: %w", name, builder.ErrDraftNotFound)
		}

		body, err := json.Marshal(draftJSON(draft))
		if err != nil {
			return nil, fmt.Errorf("marshal draft: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// NewLinodeProfileDraftDiscardTool returns the
// linode_profile_draft_discard builder tool. It removes the named
// draft from the registry. Idempotent: discarding a non-existent
// draft returns {"discarded": false} rather than an error so the
// model can call it safely during cleanup paths.
func NewLinodeProfileDraftDiscardTool(
	registry *builder.Registry,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_profile_draft_discard",
		mcp.WithDescription(
			"Discard a profile draft. Idempotent: returns "+
				`{"discarded": false} when the draft does not exist `+
				"(no error), so the model can call it from cleanup "+
				"paths without first checking existence.",
		),
		mcp.WithString(
			"name",
			mcp.Description("Draft name to discard."),
			mcp.Required(),
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

		removed := registry.Discard(name)

		body, err := json.Marshal(map[string]any{"name": name, "discarded": removed})
		if err != nil {
			return nil, fmt.Errorf("marshal discard result: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}
