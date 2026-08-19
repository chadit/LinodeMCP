package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// TestTwoStageRequestedReadsTheModeAndTheYoloGrant covers the gate a generated
// mutation runs its staged branch behind. It answers the same two questions the
// flow itself answers first, and it has to answer them the same way: a call it
// let through would run the branch's argument checks ahead of the confirm gate,
// which is not the order an ordinary call is answered in.
func TestTwoStageRequestedReadsTheModeAndTheYoloGrant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mode string
		yolo bool
		want bool
	}{
		{name: "a plan is asked for", mode: twostage.ModePlan, want: true},
		{name: "an apply is asked for", mode: twostage.ModeApply, want: true},
		{name: "no mode at all", mode: "", want: false},
		{name: "a mode naming neither stage", mode: "rehearse", want: false},
		{name: "a permitted yolo dominates a plan", mode: twostage.ModePlan, yolo: true, want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{}
			if testCase.mode != "" {
				args[keyMode] = testCase.mode
			}

			ctx := t.Context()
			if testCase.yolo {
				ctx = tools.WithYoloAllowed(ctx)
			}

			request := createRequestWithArgs(t, args)

			if got := tools.TwoStageRequested(ctx, &request); got != testCase.want {
				t.Errorf("TwoStageRequested = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestRunTwoStageWriteFallsThroughWithNoPlanStore proves the flow is the
// caller's to fall out of: a handler called directly, with no server to attach a
// store, keeps its single-step path rather than refusing a plan it cannot make.
func TestRunTwoStageWriteFallsThroughWithNoPlanStore(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModePlan})

	result, handled := tools.RunTwoStageWrite(t.Context(), &request, nil, &tools.DestructiveAction{
		ToolName: toolInstanceResize,
	})

	if handled {
		t.Errorf("the flow handled a call with no plan store, answering %v", result)
	}
}
