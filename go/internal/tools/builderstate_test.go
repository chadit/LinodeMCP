package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
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

// builderState is what the server attaches to a call: the draft registry, the
// catalog and active-profile readers, and the configuration a clone source
// resolves against. The profile arrives as its reader so a case can swap what
// the next call sees.
func builderState(
	reg *builder.Registry, catalog []profiles.ToolDescriptor,
	active func() profiles.Profile, cfg *config.Config,
) *tools.BuilderState {
	return &tools.BuilderState{
		Drafts:        reg,
		Catalog:       func() []profiles.ToolDescriptor { return catalog },
		ActiveProfile: active,
		Config:        cfg,
	}
}

// emptyConfig is the configuration the tools that resolve no profile name read.
// Present rather than nil because a partial state is treated as no state.
func emptyConfig() *config.Config {
	return &config.Config{}
}

// noProfile is the active-profile reader for the tools that never read one.
func noProfile() profiles.Profile {
	return profiles.Profile{}
}

// draftState is the state for the tools that read the registry alone.
func draftState(reg *builder.Registry) *tools.BuilderState {
	return builderState(reg, nil, noProfile, emptyConfig())
}

// catalogState is the state for the tools that read the catalog alone.
func catalogState(catalog []profiles.ToolDescriptor) *tools.BuilderState {
	return builderState(builder.NewRegistry(), catalog, noProfile, emptyConfig())
}

// builderHandler is the shape every hand-written builder factory's third
// return value has.
type builderHandler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// builderFactory is the shape of a generated builder tool's factory.
type builderFactory func(*config.Config) (mcp.Tool, profiles.Capability, tools.Handler)

// generatedBuilder is the handler one generated factory builds. A tool whose
// answer is declared rather than hooked keeps its whole body there, so this is
// the only seam a case can reach it through.
func generatedBuilder(factory builderFactory, cfg *config.Config) builderHandler {
	_, _, handler := factory(cfg)

	return handler
}

// callSaveAnswer invokes the save through its generated handler with the state
// attached and returns the parsed response.
func callSaveAnswer(
	t *testing.T, state *tools.BuilderState, args map[string]any,
) map[string]any {
	t.Helper()

	return builderBody(t, callBuilder(t, state, saveHandler(), args))
}

// saveHandler is the generated save handler, which is the only seam the save
// has now that its whole body is declared.
func saveHandler() builderHandler {
	return generatedBuilder(gentools.NewLinodeProfileDraftSaveTool, &config.Config{})
}

// wantBuilderRefusal asserts a builder handler refused with exactly the
// sentence.
func wantBuilderRefusal(
	t *testing.T, state *tools.BuilderState, handler builderHandler,
	args map[string]any, want string,
) {
	t.Helper()

	if got := refusalText(t, callBuilder(t, state, handler, args)); got != want {
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
// tool reached with no state attached refuses rather than reading from nothing.
// Production attaches the state on every call, so this is the shape a broken
// wiring change would take.
//
// Driven through the generated handler rather than an answer function because
// four of the ten no longer have one: their state read is written by the
// declaration. Each row sends the name its contract requires, since the rule
// check runs ahead of the state read and would otherwise answer first.
func TestBuilderToolsRefuseWithoutState(t *testing.T) {
	t.Parallel()

	named := map[string]any{keyName: draftFixtureName}
	// Save is gated, and the generated handler asks the gate ahead of the state
	// read, so the row has to clear it to reach what the case measures.
	confirmed := map[string]any{keyName: draftFixtureName, "confirm": true}

	for name, testCase := range map[string]struct {
		factory builderFactory
		args    map[string]any
	}{
		"linode_profile_list_tools":         {factory: gentools.NewLinodeProfileListToolsTool},
		"linode_profile_list_categories":    {factory: gentools.NewLinodeProfileListCategoriesTool},
		"linode_profile_can_run":            {factory: gentools.NewLinodeProfileCanRunTool},
		"linode_profile_draft_new":          {factory: gentools.NewLinodeProfileDraftNewTool, args: named},
		"linode_profile_draft_show":         {factory: gentools.NewLinodeProfileDraftShowTool, args: named},
		"linode_profile_draft_discard":      {factory: gentools.NewLinodeProfileDraftDiscardTool, args: named},
		"linode_profile_draft_add_tools":    {factory: gentools.NewLinodeProfileDraftAddToolsTool, args: named},
		"linode_profile_draft_remove_tools": {factory: gentools.NewLinodeProfileDraftRemoveToolsTool, args: named},
		"linode_profile_draft_set":          {factory: gentools.NewLinodeProfileDraftSetTool, args: named},
		"linode_profile_draft_save":         {factory: gentools.NewLinodeProfileDraftSaveTool, args: confirmed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := mcp.CallToolRequest{}
			request.Params.Arguments = testCase.args

			handler := generatedBuilder(testCase.factory, &config.Config{})

			result, err := handler(t.Context(), request)
			if err != nil {
				t.Fatalf("unexpected handler error: %v", err)
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
	named := map[string]any{keyName: draftFixtureName}

	callBuilder(t, state, generatedBuilder(gentools.NewLinodeProfileDraftNewTool, &config.Config{}), named)

	result := callBuilder(t, state, generatedBuilder(gentools.NewLinodeProfileDraftShowTool, nil), named)
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
		Config:        emptyConfig(),
	}

	args := map[string]any{tcCalls: []any{map[string]any{"tool": toolInstanceBoot}}}
	handler := generatedBuilder(gentools.NewLinodeProfileCanRunTool, nil)

	before := builderBody(t, callBuilder(t, state, handler, args))
	if got := before["active_profile"]; got != "before" {
		t.Errorf("active_profile = %v, want %q", got, "before")
	}

	active = profiles.Profile{Name: "after", AllowedTools: []string{toolInstanceBoot}}

	after := builderBody(t, callBuilder(t, state, handler, args))
	if got := after["active_profile"]; got != "after" {
		t.Errorf("active_profile = %v, want %q", got, "after")
	}
}

// ghostDraftName is the draft every hostile-shape case names; no draft by that
// name is ever registered, so the refusal comes from the registry lookup.
const ghostDraftName = "ghost"

// TestDraftToolPatternsSurviveHostileShapes drives the shared string-array
// reader through every shape a caller can send: an absent key, a value that
// is not a list, and a list mixing types, each parsing to what the draft
// registry then words as its own refusal.
func TestDraftToolPatternsSurviveHostileShapes(t *testing.T) {
	t.Parallel()

	for name, arguments := range map[string]map[string]any{
		"tools key absent":     {managedContactNameParam: ghostDraftName},
		"tools not a list":     {managedContactNameParam: ghostDraftName, "tools": "linode_domain_*"},
		"tools of mixed types": {managedContactNameParam: ghostDraftName, "tools": []any{1, "linode_domain_*"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := callBuilder(t,
				draftState(builder.NewRegistry()),
				generatedBuilder(gentools.NewLinodeProfileDraftAddToolsTool, &config.Config{}),
				arguments)

			if !result.IsError {
				t.Fatal("result.IsError = false, want the draft-not-found refusal")
			}
		})
	}
}
