package tools

import (
	"context"
	"fmt"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// isBuiltinProfileName reports whether the given name matches a
// built-in profile. Built-ins live in code; a user-defined entry
// with the same name would silently shadow the built-in and confuse
// the catalog. The Phase 7c clone command rejects the same names;
// the save handler reuses this check at write time.
//
// Returns by walking a local slice rather than a package-level map
// to keep the lookup table out of global state (gochecknoglobals).
// The list is tiny so the linear scan is irrelevant in practice.
func isBuiltinProfileName(name string) bool {
	return slices.Contains([]string{
		profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
		profiles.BuiltinFullAccess,
		profiles.BuiltinEmergency,
	}, name)
}

// ProfileDraftSaveAnswer answers linode_profile_draft_save. On success it
// re-reads the config fresh from disk, merges the draft into Config.Profiles,
// writes the updated config atomically, and answers the diff against the prior
// state (or against empty for a new profile).
//
// Does NOT change the active profile. After save, the user runs
// “linodemcp profile use <name>“ (or the equivalent CLI/MCP step) to switch.
//
// The file watcher picks up the rename and triggers a hot-reload via the
// existing Phase 5 plumbing, so newly-added profiles become resolvable without
// a server restart.
//
// The confirm gate runs ahead of this in the generated handler, which reports
// the contract's own confirm_message. Every refusal below answers before the
// config file is touched.
//
// The target path is read from config.Path on every call rather than captured,
// so a LINODEMCP_CONFIG_PATH override applies at call time and a concurrent
// edit is not stomped.
func ProfileDraftSaveAnswer(
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

	if isBuiltinProfileName(name) {
		return mcp.NewToolResultError("cannot save over built-in profile name: " + name), nil
	}

	draft, ok := state.Drafts.Get(name)
	if !ok {
		return mcp.NewToolResultError(draftNotFound(name)), nil
	}

	path := config.Path()

	cfg, err := config.Load(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load config from %s: %v", path, err)), nil
	}

	draftCfg := builder.DraftAsUserProfile(draft)

	var existing *config.UserProfileConfig

	if prior, hadPrior := cfg.Profiles[name]; hadPrior {
		existing = &prior
	}

	diff := builder.ComputeDiff(name, &draftCfg, existing)

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.UserProfileConfig)
	}

	cfg.Profiles[name] = draftCfg

	if writeErr := config.WriteAtomic(path, cfg); writeErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write config to %s: %v", path, writeErr)), nil
	}

	return MarshalProtoToolResponse(draftSaveProto(diff))
}

// draftSaveProto converts a save diff into its response message.
//
// ComputeDiff writes exactly three value shapes: a string for description, a
// string list for the environment and scope lists, and a bool for allow_yolo.
// Converting them with structpb's own constructors rather than through the
// fallible NewValue is what makes this total: there is no failure for the
// caller to report, so no branch it can never take.
func draftSaveProto(diff *builder.Diff) *linodev1.ProfileDraftSaveResponse {
	out := &linodev1.ProfileDraftSaveResponse{
		Name:          diff.Name,
		IsNew:         diff.IsNew,
		AddedTools:    diff.AddedTools,
		RemovedTools:  diff.RemovedTools,
		ChangedFields: make(map[string]*linodev1.ProfileFieldDiff, len(diff.ChangedFields)),
	}

	for field, change := range diff.ChangedFields {
		out.ChangedFields[field] = &linodev1.ProfileFieldDiff{
			Old: fieldDiffValue(change.Old),
			New: fieldDiffValue(change.New),
		}
	}

	return out
}

// fieldDiffValue renders one side of a changed field. The three arms are the
// three shapes ComputeDiff produces; anything else would be a field it grew
// without this being told, and reads as its own text rather than as nothing.
func fieldDiffValue(value any) *structpb.Value {
	if list, isList := value.([]string); isList {
		return stringListChange(list)
	}

	if flag, isBool := value.(bool); isBool {
		return structpb.NewBoolValue(flag)
	}

	return structpb.NewStringValue(fmt.Sprint(value))
}
