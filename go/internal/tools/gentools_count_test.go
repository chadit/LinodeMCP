package tools_test

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// countRequest wraps one call's arguments the way the MCP layer hands them to a
// handler.
func countRequest(arguments map[string]any) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: arguments}}
}

func TestArgumentLenCountsWhatTheCallerSent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      int
	}{
		{
			name:      "two entries",
			arguments: map[string]any{keyPlacementGroupLinodes: []any{1.0, 2.0}},
			want:      2,
		},
		{
			name:      "one entry",
			arguments: map[string]any{keyPlacementGroupLinodes: []any{7.0}},
			want:      1,
		},
		{
			name:      "empty list",
			arguments: map[string]any{keyPlacementGroupLinodes: []any{}},
			want:      0,
		},
		{name: "absent", arguments: map[string]any{}, want: 0},
		// A caller who sent text sent no list, and counting its characters
		// would report a number that means nothing.
		{
			name:      "not a list",
			arguments: map[string]any{keyPlacementGroupLinodes: "1,2"},
			want:      0,
		},
		{
			name:      "nil value",
			arguments: map[string]any{keyPlacementGroupLinodes: nil},
			want:      0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.ArgumentLen(countRequest(testCase.arguments), keyPlacementGroupLinodes)
			if got != testCase.want {
				t.Errorf("ArgumentLen = %d, want %d", got, testCase.want)
			}
		})
	}
}

// The prose linode_placement_group_assign has always answered with, rendered
// through the expression the emitter writes for {linodes:len} rather than the
// hand-written len() it replaces.
func TestArgumentLenFillsTheAssignedCountSentence(t *testing.T) {
	t.Parallel()

	request := countRequest(map[string]any{keyPlacementGroupLinodes: []any{1.0, 2.0}})

	got := fmt.Sprintf("Assigned %d Linode(s) to placement group %d",
		tools.ArgumentLen(request, keyPlacementGroupLinodes), 789)

	want := "Assigned 2 Linode(s) to placement group 789"
	if got != want {
		t.Errorf("rendered %q, want %q", got, want)
	}
}
