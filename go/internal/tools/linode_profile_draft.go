package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

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

// ProfileDraftNewAnswer answers linode_profile_draft_new. It starts a new
// draft in the server's draft registry. Optional “clone_from“ seeds the draft
// from an existing profile (built-in or user-defined); without it the draft
// starts empty.
//
// Refusals, each answered as a tool result:
//
//   - msgDraftNameMissing when “name“ is empty
//   - "draft already exists" when the name is already drafted
//   - "clone_from profile not found" when “clone_from“ is non-empty
//     but resolves to no profile
//
// The clone source resolves against the config the server is running and the
// catalog the call's builder state carries, so a draft cloned mid-session sees
// the tools the running server has rather than the set it started with.
func ProfileDraftNewAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	state, refusal := builderStateOrRefusal(ctx)
	if refusal != nil {
		return refusal, nil
	}

	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError(msgDraftNameMissing), nil
	}

	cloneFrom := request.GetString("clone_from", "")

	var source *profiles.Profile

	if cloneFrom != "" {
		resolved, ok := profiles.LookupProfile(cloneFrom, cfg, state.Catalog())
		if !ok {
			return mcp.NewToolResultError("clone_from profile not found: " + cloneFrom), nil
		}

		source = &resolved
	}

	// Create reports an empty name or a name the registry already holds, and
	// the empty name is refused above, so a failure here is the name already
	// being drafted. Reading the happy path first is what keeps that from
	// needing a branch no call can reach.
	draft, err := state.Drafts.Create(name, source)
	if err == nil {
		return MarshalProtoToolResponse(draftProto(draft))
	}

	return mcp.NewToolResultError("draft already exists: " + name), nil
}

// ProfileDraftShowAnswer answers linode_profile_draft_show. It reads the named
// draft and returns its current state. A miss refuses so the model can surface
// the typo or expired session.
func ProfileDraftShowAnswer(
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

	draft, ok := state.Drafts.Get(name)
	if !ok {
		return mcp.NewToolResultError(draftNotFound(name)), nil
	}

	return MarshalProtoToolResponse(draftProto(draft))
}

// ProfileDraftDiscardAnswer answers linode_profile_draft_discard. It removes
// the named draft from the registry. Idempotent: discarding a draft that is
// not there answers {"discarded": false} rather than a refusal, so the model
// can call it from cleanup paths without first checking existence.
func ProfileDraftDiscardAnswer(
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

	removed := state.Drafts.Discard(name)

	return MarshalProtoToolResponse(&linodev1.ProfileDraftDiscardResponse{
		Name:      name,
		Discarded: removed,
	})
}
