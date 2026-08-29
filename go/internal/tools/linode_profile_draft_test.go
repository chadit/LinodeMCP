package tools_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	// draftFixtureName is the conventional name reused across happy-path
	// tests for new/show/discard. Hoisted as a constant per goconst.
	draftFixtureName = "dns-readall"
	// cloneSourceName is the resolver-fixture profile callers can clone
	// from in the clone_from path tests.
	cloneSourceName = "compute-admin"
)

// cloneFixtureCatalog is the catalog a cloned draft's tool patterns expand
// against, which is what the seam hands the resolver.
func cloneFixtureCatalog() []profiles.ToolDescriptor {
	return []profiles.ToolDescriptor{
		{Name: toolInstanceBoot, Capability: profiles.CapWrite},
		{Name: canRunReadTool, Capability: profiles.CapRead},
		{Name: tcLinodeDomainGet, Capability: profiles.CapRead},
	}
}

// cloneFixtureConfig carries one user-defined profile under cloneSourceName,
// which is what _draft_new resolves clone_from against. User-defined entries
// shadow built-ins, so the fixture's fields are the ones a clone lands on.
func cloneFixtureConfig() *config.Config {
	return &config.Config{
		Profiles: map[string]config.UserProfileConfig{
			cloneSourceName: {
				Description:         "Compute admin clone source",
				AllowedTools:        []string{toolInstanceBoot, canRunReadTool},
				AllowedEnvironments: []string{envProd},
				RequiredTokenScopes: []string{scopeLinodesReadWrite},
			},
		},
	}
}

// fixtureSourceProfile is the Profile cloneFixtureConfig resolves to. Its
// AllowedTools are sorted because pattern expansion sorts what it matched.
func fixtureSourceProfile() profiles.Profile {
	return profiles.Profile{
		Name:                cloneSourceName,
		Description:         "Compute admin clone source",
		AllowedTools:        []string{toolInstanceBoot, canRunReadTool},
		AllowedEnvironments: []string{envProd},
		RequiredTokenScopes: []string{scopeLinodesReadWrite},
		AllowYolo:           false,
	}
}

// cloneState carries the registry, the catalog the clone resolves against, and
// the configuration the clone source is looked up in.
func cloneState(reg *builder.Registry) *tools.BuilderState {
	return builderState(reg, cloneFixtureCatalog(), noProfile, cloneFixtureConfig())
}

// callDraftHandler invokes one generated draft handler with the state attached
// and returns the parsed JSON object.
func callDraftHandler(
	t *testing.T,
	state *tools.BuilderState,
	handler builderHandler,
	args map[string]any,
) map[string]any {
	t.Helper()

	return builderBody(t, callBuilder(t, state, handler, args))
}

// draftNewHandler, draftShowHandler and draftDiscardHandler are the generated
// handlers the three lifecycle tools answer through, which is where their whole
// body sits now that each declares its answer.
func draftNewHandler() builderHandler {
	return generatedBuilder(gentools.NewLinodeProfileDraftNewTool, nil)
}

func draftShowHandler() builderHandler {
	return generatedBuilder(gentools.NewLinodeProfileDraftShowTool, nil)
}

func draftDiscardHandler() builderHandler {
	return generatedBuilder(gentools.NewLinodeProfileDraftDiscardTool, nil)
}

// TestDraftNewRegistration locks in the static contract: the tool's
// name, description presence, and CapMeta tag. CapMeta is what makes
// the builder tools always-available regardless of the active profile.
func TestDraftNewRegistration(t *testing.T) {
	t.Parallel()

	tool, capability, handler := gentools.NewLinodeProfileDraftNewTool(cloneFixtureConfig())

	if tool.Name != "linode_profile_draft_new" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_draft_new")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapMeta {
		t.Errorf("capability = %v, want %v", capability, profiles.CapMeta)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

// TestDraftNewCreatesEmptyDraft is the no-clone-from happy path.
// Resulting draft carries the requested name and empty slices/zero
// flag for everything else.
func TestDraftNewCreatesEmptyDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	out := callDraftHandler(t, cloneState(reg), draftNewHandler(),
		map[string]any{keyName: draftFixtureName})

	if !reflect.DeepEqual(out[keyName], draftFixtureName) {
		t.Errorf("out[keyName] = %v, want %v", out[keyName], draftFixtureName)
	}

	if v := out[keyDescription]; v != nil && v != "" {
		t.Errorf("value = %v, want empty", out[keyDescription])
	}

	for key, want := range map[string]any{
		tcAllowedTools:        []any{},
		tcAllowedEnvironments: []any{},
		tcRequiredTokenScopes: []any{},
		"allow_yolo":          false,
	} {
		if !reflect.DeepEqual(out[key], want) {
			t.Errorf("out[%v] = %v, want %v", key, out[key], want)
		}
	}

	// Registry side-effect: the draft is now retrievable.
	_, ok := reg.Get(draftFixtureName)
	if !ok {
		t.Error("ok = false, want true")
	}
}

// TestDraftNewClonesFromSource covers the clone_from path: every
// field on the source profile lands on the new draft.
func TestDraftNewClonesFromSource(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	out := callDraftHandler(t, cloneState(reg), draftNewHandler(),
		map[string]any{
			keyName:      draftFixtureName,
			"clone_from": cloneSourceName,
		})

	src := fixtureSourceProfile()

	for key, want := range map[string]any{
		keyName:               draftFixtureName,
		keyDescription:        src.Description,
		tcAllowedTools:        anySlice(src.AllowedTools),
		tcAllowedEnvironments: anySlice(src.AllowedEnvironments),
		tcRequiredTokenScopes: anySlice(src.RequiredTokenScopes),
		"allow_yolo":          src.AllowYolo,
	} {
		if !reflect.DeepEqual(out[key], want) {
			t.Errorf("out[%v] = %v, want %v", key, out[key], want)
		}
	}
}

// TestDraftNewRefusesMissingName covers the validation guard. The
// schema marks name as required so MCP should reject before the
// handler runs, but we belt-and-suspenders inside the handler too.
func TestDraftNewRefusesMissingName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantRefusal(t, cloneState(reg), draftNewHandler(),
		nil, wantDraftNameMissing)
}

// TestDraftNewRefusesUnknownCloneSource covers the unknown-source path.
// The user typo'd a profile name; surface the error rather than
// silently producing an empty draft.
func TestDraftNewRefusesUnknownCloneSource(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	wantRefusal(t, cloneState(reg), draftNewHandler(),
		map[string]any{
			keyName:      draftFixtureName,
			"clone_from": "nonexistent-profile",
		}, "clone_from profile not found: nonexistent-profile")

	_, exists := reg.Get(draftFixtureName)
	if exists {
		t.Error("exists = true, want false")
	}
}

// TestDraftNewRefusesDuplicateName surfaces the underlying
// builder.ErrDraftExists. The user must discard first or pick a
// different name; no silent overwrite.
func TestDraftNewRefusesDuplicateName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	state := cloneState(reg)

	_ = callDraftHandler(t, state, draftNewHandler(),
		map[string]any{keyName: draftFixtureName})

	wantRefusal(t, state, draftNewHandler(),
		map[string]any{keyName: draftFixtureName}, "draft already exists: dns-readall")
}

// TestDraftShowReturnsLiveDraftState reads the draft back. Mirrors
// the conversation flow where the model creates a draft, mutates it
// (Phase 8.4), then re-reads to confirm.
func TestDraftShowReturnsLiveDraftState(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	src := fixtureSourceProfile()

	_, err := reg.Create(draftFixtureName, &src)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callDraftHandler(t, draftState(reg), draftShowHandler(),
		map[string]any{keyName: draftFixtureName})

	for key, want := range map[string]any{
		keyName:        draftFixtureName,
		keyDescription: src.Description,
		tcAllowedTools: anySlice(src.AllowedTools),
	} {
		if !reflect.DeepEqual(out[key], want) {
			t.Errorf("out[%v] = %v, want %v", key, out[key], want)
		}
	}
}

// TestDraftShowRefusesUnknown covers the typo / expired-session path. The
// handler answers the miss as a tool result the model can read and correct
// from, which is the sentence Python answers too.
func TestDraftShowRefusesUnknown(t *testing.T) {
	t.Parallel()

	wantRefusal(t, draftState(builder.NewRegistry()), draftShowHandler(),
		map[string]any{keyName: draftNonexistent}, "draft not found: nonexistent-draft")
}

// TestDraftShowRefusesMissingName mirrors the _new validation guard.
func TestDraftShowRefusesMissingName(t *testing.T) {
	t.Parallel()

	wantRefusal(t, draftState(builder.NewRegistry()), draftShowHandler(),
		nil, wantDraftNameMissing)
}

// TestDraftDiscardRemovesDraft is the happy path. The discarded
// response carries the boolean and the name for human-readable logs;
// the registry no longer holds the draft afterward.
func TestDraftDiscardRemovesDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create(draftFixtureName, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	out := callDraftHandler(t, draftState(reg), draftDiscardHandler(),
		map[string]any{keyName: draftFixtureName})

	if !reflect.DeepEqual(out[keyName], draftFixtureName) {
		t.Errorf("out[keyName] = %v, want %v", out[keyName], draftFixtureName)
	}

	if !reflect.DeepEqual(out["discarded"], true) {
		t.Errorf("got %v, want %v", out["discarded"], true)
	}

	_, exists := reg.Get(draftFixtureName)
	if exists {
		t.Error("exists = true, want false")
	}
}

// TestDraftDiscardIdempotent covers the unknown-name path. Discard
// against a name that was never created returns discarded=false, not
// an error. Tool handlers should be safe to call on cleanup paths.
func TestDraftDiscardIdempotent(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	out := callDraftHandler(t, draftState(reg), draftDiscardHandler(),
		map[string]any{keyName: draftNonexistent})

	if !reflect.DeepEqual(out[keyName], draftNonexistent) {
		t.Errorf("out[keyName] = %v, want %v", out[keyName], draftNonexistent)
	}

	if !reflect.DeepEqual(out["discarded"], false) {
		t.Errorf("got %v, want %v", out["discarded"], false)
	}
}

// TestDraftDiscardRefusesMissingName mirrors _new and _show.
func TestDraftDiscardRefusesMissingName(t *testing.T) {
	t.Parallel()

	wantRefusal(t, draftState(builder.NewRegistry()), draftDiscardHandler(),
		nil, wantDraftNameMissing)
}

// TestDraftToolsRespectContextCancellation locks the cancellation
// contract across all three handlers. A canceled context surfaces
// ctx.Err and produces no result. Test exists to catch a refactor
// that drops the select gate.
func TestDraftToolsRespectContextCancellation(t *testing.T) {
	t.Parallel()

	_, _, newHandler := gentools.NewLinodeProfileDraftNewTool(cloneFixtureConfig())
	_, _, showHandler := gentools.NewLinodeProfileDraftShowTool(cloneFixtureConfig())
	_, _, discardHandler := gentools.NewLinodeProfileDraftDiscardTool(cloneFixtureConfig())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, errNew := newHandler(ctx, mcp.CallToolRequest{})
	_, errShow := showHandler(ctx, mcp.CallToolRequest{})
	_, errDiscard := discardHandler(ctx, mcp.CallToolRequest{})

	if !errors.Is(errNew, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errNew)
	}

	if !errors.Is(errShow, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errShow)
	}

	if !errors.Is(errDiscard, context.Canceled) {
		t.Fatalf("expected error %v, got %v", context.Canceled, errDiscard)
	}
}

// anySlice converts a []string to the []any shape json.Unmarshal
// produces for arrays. Equality assertions in this file compare
// against the unmarshaled wire shape, so the conversion lives here
// rather than at every call site.
func anySlice(in []string) []any {
	out := make([]any, len(in))
	for i, item := range in {
		out[i] = item
	}

	return out
}
