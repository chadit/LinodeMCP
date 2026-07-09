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
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// ConfigPathProvider returns the path of the active config file.
// The Phase 8.5 `_draft_save` handler uses this to re-read the
// config fresh from disk (avoiding races with concurrent edits) and
// to pass the same path to `config.WriteAtomic`. Provided as a
// function so tests can supply a reproducible stand-in.
type ConfigPathProvider func() string

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

// NewLinodeProfileDraftSaveTool returns the linode_profile_draft_save
// builder tool. Requires “confirm: true“ (matching every other
// confirmation-gated write tool). On success: re-reads the config
// fresh from disk, merges the draft into Config.Profiles, writes the
// updated config atomically, and returns the diff against the prior
// state (or against empty for a new profile).
//
// Does NOT change the active profile. After save, the user runs
// “linodemcp profile use <name>“ (or the equivalent CLI/MCP step)
// to switch.
//
// The file watcher picks up the rename and triggers a hot-reload via
// the existing Phase 5 plumbing, so newly-added profiles become
// resolvable without a server restart.
func NewLinodeProfileDraftSaveTool(
	registry *builder.Registry,
	configPath ConfigPathProvider,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_draft_save",
		"Save a profile draft to the config file. Requires confirm=true. "+
			"Computes the diff against the prior user-defined profile "+
			"with the same name (or against empty for a new profile) "+
			"and returns it in the response so the model can summarize. "+
			"Does NOT change the active profile; the user runs "+
			"`linodemcp profile use <name>` separately. Saving over a "+
			"built-in profile name is refused.",
		toolschemas.Schema("linode.mcp.v1.ProfileDraftSaveInput"),
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

		if !request.GetBool("confirm", false) {
			return nil, ErrConfirmRequired
		}

		if isBuiltinProfileName(name) {
			return nil, fmt.Errorf("%w: %s", ErrSaveBuiltinName, name)
		}

		draft, ok := registry.Get(name)
		if !ok {
			return nil, fmt.Errorf("draft %q: %w", name, builder.ErrDraftNotFound)
		}

		path := configPath()
		if path == "" {
			return nil, ErrConfigPathUnknown
		}

		cfg, err := config.Load(path)
		if err != nil {
			return nil, fmt.Errorf("load config from %q: %w", path, err)
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

		if err := config.WriteAtomic(path, cfg); err != nil {
			return nil, fmt.Errorf("write config to %q: %w", path, err)
		}

		saveProto, err := draftSaveProto(diff)
		if err != nil {
			return nil, err
		}

		return MarshalProtoToolResponse(saveProto)
	}

	return tool, profiles.CapMeta, handler
}

// draftSaveProto converts a save diff into its response message. The old/new
// field values are free-form (string, bool, or string array per field), so
// they round-trip through structpb values.
func draftSaveProto(diff *builder.Diff) (*linodev1.ProfileDraftSaveResponse, error) {
	out := &linodev1.ProfileDraftSaveResponse{
		Name:          diff.Name,
		IsNew:         diff.IsNew,
		AddedTools:    diff.AddedTools,
		RemovedTools:  diff.RemovedTools,
		ChangedFields: make(map[string]*linodev1.ProfileFieldDiff, len(diff.ChangedFields)),
	}

	for field, change := range diff.ChangedFields {
		oldValue, err := structpb.NewValue(widenForStructValue(change.Old))
		if err != nil {
			return nil, fmt.Errorf("convert old value for %s: %w", field, err)
		}

		newValue, err := structpb.NewValue(widenForStructValue(change.New))
		if err != nil {
			return nil, fmt.Errorf("convert new value for %s: %w", field, err)
		}

		out.ChangedFields[field] = &linodev1.ProfileFieldDiff{Old: oldValue, New: newValue}
	}

	return out, nil
}

// widenForStructValue converts the []string diff values (environment and
// scope lists) to []any, the only slice type structpb.NewValue accepts.
// Scalars pass through unchanged.
func widenForStructValue(value any) any {
	if strings, ok := value.([]string); ok {
		return stringsToAnySlice(strings)
	}

	return value
}
