package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The profile-builder tools answer from the server state their call carries
// rather than from a Linode route, so each one's whole result is its own
// answer hook. The bodies stay in the tools package because that is where the
// builder state lives; these are the names the generated factories call.

// LinodeProfileListToolsAnswer enumerates every registerable tool with its
// capability and categories, name-sorted, filtered by the optional category
// and capability arguments.
func LinodeProfileListToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileListToolsAnswer(ctx, request, cfg))
}

// LinodeProfileListCategoriesAnswer reduces the catalog to its deduplicated
// category list with per-category tool counts, sorted by name.
func LinodeProfileListCategoriesAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileListCategoriesAnswer(ctx, request, cfg))
}

// LinodeProfileCanRunAnswer pre-checks a sequence of calls against the active
// profile and answers a per-call verdict with a summary.
func LinodeProfileCanRunAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileCanRunAnswer(ctx, request, cfg))
}

// LinodeProfileDraftNewAnswer starts a draft, optionally seeded from the
// profile clone_from names. It is the one builder hook that reads the
// configuration, because that is where the clone source is resolved from.
func LinodeProfileDraftNewAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftNewAnswer(ctx, request, cfg))
}

// LinodeProfileDraftShowAnswer reads one draft's current state.
func LinodeProfileDraftShowAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftShowAnswer(ctx, request, cfg))
}

// LinodeProfileDraftDiscardAnswer removes one draft, answering whether it was
// there to remove.
func LinodeProfileDraftDiscardAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftDiscardAnswer(ctx, request, cfg))
}

// LinodeProfileDraftAddToolsAnswer expands the call's patterns against the live
// catalog and merges what matched into the draft.
func LinodeProfileDraftAddToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftAddToolsAnswer(ctx, request, cfg))
}

// LinodeProfileDraftRemoveToolsAnswer strips the tools the call's patterns
// match on the draft's own list.
func LinodeProfileDraftRemoveToolsAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftRemoveToolsAnswer(ctx, request, cfg))
}

// LinodeProfileDraftSetAnswer sets the draft's optional fields and answers
// which ones changed.
func LinodeProfileDraftSetAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftSetAnswer(ctx, request, cfg))
}

// LinodeProfileDraftSaveAnswer merges the draft into the config file and
// answers the diff. The generated handler runs the confirm gate ahead of it.
func LinodeProfileDraftSaveAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.ProfileDraftSaveAnswer(ctx, request, cfg))
}
