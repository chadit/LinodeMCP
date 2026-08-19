package toolhooks_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// answerHook is the shape the meta tier calls every answer hook through.
type answerHook func(context.Context, *mcp.CallToolRequest, *config.Config) (*mcp.CallToolResult, error)

// TestAnswerHooksForwardTheirReaders covers the meta surface's own seam: each
// hook hands the tier the result its reader assembled, canonical JSON and all.
// What each reader answers is pinned beside it in internal/tools; what is
// pinned here is that the hook reaches it and passes the answer through.
func TestAnswerHooksForwardTheirReaders(t *testing.T) {
	for _, testCase := range []struct {
		name string
		hook answerHook
		args map[string]any
		want string
	}{
		{
			name: "version",
			hook: toolhooks.VersionAnswer,
			want: "platform",
		},
		{
			name: "audit health",
			hook: toolhooks.LinodeAuditHealthAnswer,
			want: "jsonl_path",
		},
		{
			name: "audit recent",
			hook: toolhooks.LinodeAuditRecentAnswer,
			want: "count",
		},
		{
			name: "audit summary",
			hook: toolhooks.LinodeAuditSummaryAnswer,
			want: "total_events",
		},
		{
			name: "audit export",
			hook: toolhooks.LinodeAuditExportAnswer,
			args: map[string]any{"format": "json"},
			want: "record_count",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// The audit readers resolve their store from the environment, and
			// t.Setenv is what keeps them off the developer's own audit log.
			// It rules out t.Parallel here and in the parent.
			t.Setenv("XDG_STATE_HOME", t.TempDir())

			result, err := testCase.hook(t.Context(), requestFor(testCase.args), &config.Config{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				t.Fatalf("result.IsError = true, want false")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !json.Valid([]byte(textContent.Text)) {
				t.Fatalf("answer is not JSON:\n%s", textContent.Text)
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("answer does not carry %v:\n%s", testCase.want, textContent.Text)
			}
		})
	}
}

// TestAuditReportAnswerForwardsTheRefusal is the report tool's half of the same
// seam. It is separate because the report hook answers an error result for a
// name no catalog holds, and an empty catalog is what a blank config carries.
func TestAuditReportAnswerForwardsTheRefusal(t *testing.T) {
	t.Parallel()

	result, err := toolhooks.LinodeAuditReportAnswer(
		t.Context(), requestFor(map[string]any{argName: "does-not-exist"}), &config.Config{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "unknown report") {
		t.Errorf("answer does not name the cause:\n%s", textContent.Text)
	}
}

// TestAnswerHooksStopOnAClosedContext covers the two hooks whose readers take
// no context of their own, so cancellation is theirs to honor.
func TestAnswerHooksStopOnAClosedContext(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		hook answerHook
		name string
	}{
		{hook: toolhooks.VersionAnswer, name: "version"},
		{hook: toolhooks.LinodeAuditRecentAnswer, name: "audit recent"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			if _, err := testCase.hook(ctx, requestFor(nil), &config.Config{}); err == nil {
				t.Error("err = nil, want a cancellation")
			}
		})
	}
}
