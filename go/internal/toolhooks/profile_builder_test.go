package toolhooks_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	// bootTool is the one catalog entry the builder fixtures compose against.
	bootTool = "linode_instance_boot"
	// fixtureDraft is the draft the mutator cases act on, created before the
	// table runs so no case depends on another having gone first.
	fixtureDraft = "fixture-draft"
)

// TestProfileBuilderHooksForwardTheirAnswers covers the builder half of the
// meta seam: each hook hands back what the answer in internal/tools assembled.
// What each answer says is pinned beside it there; what is pinned here is that
// the hook reaches it and passes the result through untouched.
func TestProfileBuilderHooksForwardTheirAnswers(t *testing.T) {
	t.Parallel()

	// The mutator cases need a draft that is already there. Creating it up
	// front rather than leaning on the draft_new case is what keeps the
	// subtests independent: they run in parallel, so an order dependency
	// between them would fail whenever the schedule changed.
	registry := builder.NewRegistry()
	if _, err := registry.Create(fixtureDraft, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := &tools.BuilderState{
		Drafts: registry,
		Catalog: func() []profiles.ToolDescriptor {
			return []profiles.ToolDescriptor{
				{Name: bootTool, Capability: profiles.CapWrite},
			}
		},
		ActiveProfile: func() profiles.Profile {
			return profiles.Profile{Name: "fixture", AllowedTools: []string{bootTool}}
		},
	}

	for _, testCase := range []struct {
		hook answerHook
		args map[string]any
		name string
		want string
	}{
		{
			name: "list tools",
			hook: toolhooks.LinodeProfileListToolsAnswer,
			want: bootTool,
		},
		{
			name: "list categories",
			hook: toolhooks.LinodeProfileListCategoriesAnswer,
			want: "tool_count",
		},
		{
			name: "can run",
			hook: toolhooks.LinodeProfileCanRunAnswer,
			args: map[string]any{"calls": []any{map[string]any{"tool": bootTool}}},
			want: "active_profile",
		},
		{
			name: "draft new",
			hook: toolhooks.LinodeProfileDraftNewAnswer,
			args: map[string]any{argName: "seam-draft"},
			want: "allowed_tools",
		},
		{
			name: "draft discard",
			hook: toolhooks.LinodeProfileDraftDiscardAnswer,
			args: map[string]any{argName: "never-drafted"},
			want: "discarded",
		},
		{
			name: "draft set",
			hook: toolhooks.LinodeProfileDraftSetAnswer,
			args: map[string]any{argName: fixtureDraft, "allow_yolo": true},
			want: "changes",
		},
		{
			name: "draft add tools",
			hook: toolhooks.LinodeProfileDraftAddToolsAnswer,
			args: map[string]any{argName: fixtureDraft, "tools": []any{bootTool}},
			want: "added",
		},
		{
			name: "draft remove tools",
			hook: toolhooks.LinodeProfileDraftRemoveToolsAnswer,
			args: map[string]any{argName: fixtureDraft, "tools": []any{bootTool}},
			want: "removed",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := mcp.CallToolRequest{}
			request.Params.Arguments = testCase.args

			ctx := tools.WithBuilderState(t.Context(), state)

			result, err := testCase.hook(ctx, &request, &config.Config{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				t.Fatalf("result.IsError = true, want false")
			}

			text, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("result content must be TextContent")
			}

			if !strings.Contains(text.Text, testCase.want) {
				t.Errorf("answer does not carry %q:\n%s", testCase.want, text.Text)
			}
		})
	}
}

// TestProfileBuilderHooksForwardTheRefusal covers the other direction: a call
// with no builder state attached comes back as the tool-result refusal rather
// than as a transport error, which is what the generated handler passes on.
func TestProfileBuilderHooksForwardTheRefusal(t *testing.T) {
	t.Parallel()

	const want = "draft registry not configured"

	for _, testCase := range []struct {
		hook answerHook
		name string
	}{
		{name: "list tools", hook: toolhooks.LinodeProfileListToolsAnswer},
		{name: "list categories", hook: toolhooks.LinodeProfileListCategoriesAnswer},
		{name: "can run", hook: toolhooks.LinodeProfileCanRunAnswer},
		{name: "draft new", hook: toolhooks.LinodeProfileDraftNewAnswer},
		{name: "draft show", hook: toolhooks.LinodeProfileDraftShowAnswer},
		{name: "draft discard", hook: toolhooks.LinodeProfileDraftDiscardAnswer},
		{name: "draft set", hook: toolhooks.LinodeProfileDraftSetAnswer},
		{name: "draft add tools", hook: toolhooks.LinodeProfileDraftAddToolsAnswer},
		{name: "draft remove tools", hook: toolhooks.LinodeProfileDraftRemoveToolsAnswer},
		{name: "draft save", hook: toolhooks.LinodeProfileDraftSaveAnswer},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := testCase.hook(t.Context(), &mcp.CallToolRequest{}, &config.Config{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatal("result.IsError = false, want true")
			}

			text, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("result content must be TextContent")
			}

			if text.Text != want {
				t.Errorf("refusal = %q, want %q", text.Text, want)
			}
		})
	}
}
