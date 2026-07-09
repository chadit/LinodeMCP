package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// ProfileResolver returns a Profile by name across both built-in and
// user-defined catalogs. The Phase 8.3 _draft_new tool uses this to
// seed a new draft from the named clone_from profile. Returning
// (zero, false) means "no such profile".
type ProfileResolver func(name string) (profiles.Profile, bool)

// draftProto converts a builder.Draft into its response message. The
// canonical serializer emits empty repeated fields as “[]“ not “null“,
// preserving the draft JSON contract of arrays over null.
func draftProto(draft *builder.Draft) *linodev1.ProfileDraftResponse {
	return &linodev1.ProfileDraftResponse{
		Name:                draft.Name,
		Description:         draft.Description,
		AllowedTools:        draft.AllowedTools,
		AllowedEnvironments: draft.AllowedEnvironments,
		RequiredTokenScopes: draft.RequiredTokenScopes,
		AllowYolo:           draft.AllowYolo,
	}
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
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_new",
		"Start a new profile draft in the server's in-memory builder "+
			"registry. Optional clone_from seeds the draft from an "+
			"existing built-in or user-defined profile. The draft "+
			"persists only for this server's lifetime; use "+
			"linode_profile_draft_save (Phase 8.5) to write it to "+
			"the config file.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftNewInput"),
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

		return MarshalProtoToolResponse(draftProto(draft))
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
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_show",
		"Show the current state of a profile draft. Returns name, "+
			"description, allowed tools, allowed environments, "+
			"required token scopes, and the allow_yolo flag.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftShowInput"),
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

		return MarshalProtoToolResponse(draftProto(draft))
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
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_discard",
		"Discard a profile draft. Idempotent: returns "+
			`{"discarded": false} when the draft does not exist `+
			"(no error), so the model can call it from cleanup "+
			"paths without first checking existence.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftDiscardInput"),
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

		return MarshalProtoToolResponse(&linodev1.ProfileDraftDiscardResponse{
			Name:      name,
			Discarded: removed,
		})
	}

	return tool, profiles.CapMeta, handler
}
