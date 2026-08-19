package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	// wantBuilderUnconfigured is the sentence every builder tool answers when
	// the server attached no state. Python answers the same words.
	wantBuilderUnconfigured = "draft registry not configured"
	// wantDraftNameMissing is the sentence the seven name-taking builder tools
	// answer for an absent name, in both languages.
	wantDraftNameMissing = "name argument is required"
)

// builderState is what the server attaches to a call: the draft registry plus
// the catalog and active-profile readers. The profile arrives as its reader so
// a case can swap what the next call sees.
func builderState(
	reg *builder.Registry, catalog []profiles.ToolDescriptor, active func() profiles.Profile,
) *tools.BuilderState {
	return &tools.BuilderState{
		Drafts:        reg,
		Catalog:       func() []profiles.ToolDescriptor { return catalog },
		ActiveProfile: active,
	}
}

// noProfile is the active-profile reader for the tools that never read one.
func noProfile() profiles.Profile {
	return profiles.Profile{}
}

// draftState is the state for the tools that read the registry alone.
func draftState(reg *builder.Registry) *tools.BuilderState {
	return builderState(reg, nil, noProfile)
}

// catalogState is the state for the tools that read the catalog alone.
func catalogState(catalog []profiles.ToolDescriptor) *tools.BuilderState {
	return builderState(builder.NewRegistry(), catalog, noProfile)
}

// builderHandler is the shape every hand-written builder factory's third
// return value has.
type builderHandler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// builderAnswer is the shape of the answer functions the generated builder
// tools reach through their hooks.
type builderAnswer func(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error)

// callAnswer runs one builder answer with the given state attached. cfg is nil
// for every answer but draft_new's, which resolves its clone source from it.
func callAnswer(
	t *testing.T, state *tools.BuilderState, answer builderAnswer,
	cfg *config.Config, args map[string]any,
) *mcp.CallToolResult {
	t.Helper()

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := answer(tools.WithBuilderState(t.Context(), state), req, cfg)
	if err != nil {
		t.Fatalf("unexpected answer error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil answer result")
	}

	return result
}

// callSaveAnswer invokes the save answer with the state attached and returns
// the parsed response.
func callSaveAnswer(
	t *testing.T, state *tools.BuilderState, args map[string]any,
) map[string]any {
	t.Helper()

	return builderBody(t, callAnswer(t, state, tools.ProfileDraftSaveAnswer, nil, args))
}

// wantAnswerRefusal asserts a builder answer refused with exactly the sentence.
func wantAnswerRefusal(
	t *testing.T, state *tools.BuilderState, answer builderAnswer,
	cfg *config.Config, args map[string]any, want string,
) {
	t.Helper()

	if got := refusalText(t, callAnswer(t, state, answer, cfg, args)); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}
}

// callBuilder runs one builder handler with the given state attached and
// returns the result, failing on a transport error the tools do not use.
func callBuilder(
	t *testing.T, state *tools.BuilderState, handler builderHandler, args map[string]any,
) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	result, err := handler(tools.WithBuilderState(t.Context(), state), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil handler result")
	}

	return result
}

// builderBody asserts the result is an answer and returns its parsed JSON.
func builderBody(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	if result.IsError {
		t.Fatalf("result.IsError = true, want false: %s", refusalText(t, result))
	}

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("result content must be TextContent")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal response JSON: %v", err)
	}

	return out
}

// refusalText asserts the result is a refusal and returns its sentence.
func refusalText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	if len(result.Content) == 0 {
		t.Fatal("expected refusal content")
	}

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("refusal content must be TextContent")
	}

	return text.Text
}

// wantRefusal asserts a handler answered exactly the given sentence.
func wantRefusal(
	t *testing.T, state *tools.BuilderState, handler builderHandler, args map[string]any, want string,
) {
	t.Helper()

	if got := refusalText(t, callBuilder(t, state, handler, args)); got != want {
		t.Errorf("refusal = %q, want %q", got, want)
	}
}

// TestBuilderToolsRefuseWithoutState covers what no fixture can see: a builder
// answer reached with no state attached refuses rather than reading from
// nothing. Production attaches the state on every call, so this is the shape a
// broken wiring change would take. All ten are here now that the class is
// generated; the separate answers-only case it used to share this file with
// went away with the last hand-written factory.
func TestBuilderToolsRefuseWithoutState(t *testing.T) {
	t.Parallel()

	answers := map[string]builderAnswer{
		"linode_profile_list_tools":         tools.ProfileListToolsAnswer,
		"linode_profile_list_categories":    tools.ProfileListCategoriesAnswer,
		"linode_profile_can_run":            tools.ProfileCanRunAnswer,
		"linode_profile_draft_new":          tools.ProfileDraftNewAnswer,
		"linode_profile_draft_show":         tools.ProfileDraftShowAnswer,
		"linode_profile_draft_discard":      tools.ProfileDraftDiscardAnswer,
		"linode_profile_draft_add_tools":    tools.ProfileDraftAddToolsAnswer,
		"linode_profile_draft_remove_tools": tools.ProfileDraftRemoveToolsAnswer,
		"linode_profile_draft_set":          tools.ProfileDraftSetAnswer,
		"linode_profile_draft_save":         tools.ProfileDraftSaveAnswer,
	}

	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result, err := answer(t.Context(), &mcp.CallToolRequest{}, &config.Config{})
			if err != nil {
				t.Fatalf("unexpected answer error: %v", err)
			}

			if got := refusalText(t, result); got != wantBuilderUnconfigured {
				t.Errorf("refusal = %q, want %q", got, wantBuilderUnconfigured)
			}
		})
	}
}

// TestBuilderStateFromContextAnswersWhatWasAttached covers the seam itself:
// what goes in comes out, and a context with nothing attached answers nil.
func TestBuilderStateFromContextAnswersWhatWasAttached(t *testing.T) {
	t.Parallel()

	if got := tools.BuilderStateFromContext(t.Context()); got != nil {
		t.Errorf("BuilderStateFromContext(bare) = %v, want nil", got)
	}

	state := draftState(builder.NewRegistry())

	got := tools.BuilderStateFromContext(tools.WithBuilderState(t.Context(), state))
	if got != state {
		t.Errorf("BuilderStateFromContext = %v, want the attached state", got)
	}
}

// TestDraftSurvivesAcrossCalls covers the pointer-stability the design turns
// on: one state carried by two calls means a draft started by the first is
// there for the second. A state rebuilt per call would lose it.
func TestDraftSurvivesAcrossCalls(t *testing.T) {
	t.Parallel()

	state := draftState(builder.NewRegistry())

	callAnswer(t, state, tools.ProfileDraftNewAnswer, &config.Config{}, map[string]any{keyName: draftFixtureName})

	result := callAnswer(t, state, tools.ProfileDraftShowAnswer, nil, map[string]any{keyName: draftFixtureName})
	if result.IsError {
		t.Fatalf("show after new refused: %s", refusalText(t, result))
	}
}

// TestCanRunReadsTheProfileAtCallTime covers the freshness the seam's function
// members buy: swapping the profile the reader answers with changes the next
// call's verdict, which is what makes a hot reload reach an already-registered
// tool.
func TestCanRunReadsTheProfileAtCallTime(t *testing.T) {
	t.Parallel()

	active := profiles.Profile{Name: "before", AllowedTools: nil}
	catalog := []profiles.ToolDescriptor{{Name: toolInstanceBoot, Capability: profiles.CapWrite}}

	state := &tools.BuilderState{
		Drafts:        builder.NewRegistry(),
		Catalog:       func() []profiles.ToolDescriptor { return catalog },
		ActiveProfile: func() profiles.Profile { return active },
	}

	req := &mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"calls": []any{map[string]any{"tool": toolInstanceBoot}}}
	ctx := tools.WithBuilderState(t.Context(), state)

	before, err := tools.ProfileCanRunAnswer(ctx, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := builderBody(t, before)["active_profile"]; got != "before" {
		t.Errorf("active_profile = %v, want %q", got, "before")
	}

	active = profiles.Profile{Name: "after", AllowedTools: []string{toolInstanceBoot}}

	after, err := tools.ProfileCanRunAnswer(ctx, req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := builderBody(t, after)["active_profile"]; got != "after" {
		t.Errorf("active_profile = %v, want %q", got, "after")
	}
}
