package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestPlacementGroupLabelMessageAnswersTheSharedPatternSentence covers the
// shared label check directly: the placement update hook is its remaining
// caller since the create tool's label rule moved onto the contract, and a
// wording drift here would resurface through that hook.
func TestPlacementGroupLabelMessageAnswersTheSharedPatternSentence(t *testing.T) {
	t.Parallel()

	pattern := "label must start and end with an alphanumeric character and contain only " +
		"alphanumeric characters, hyphens, underscores, or periods"

	cases := []struct {
		name  string
		label string
		want  string
	}{
		{name: "a good label passes", label: "pg-test.1", want: ""},
		{name: "a leading hyphen is refused", label: "-bad", want: pattern},
		{name: "a trailing period is refused", label: "bad.", want: pattern},
		{name: "an inner space is refused", label: "bad label", want: pattern},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := tools.PlacementGroupLabelMessage(testCase.label); got != testCase.want {
				t.Errorf("PlacementGroupLabelMessage(%q) = %q, want %q", testCase.label, got, testCase.want)
			}
		})
	}
}
