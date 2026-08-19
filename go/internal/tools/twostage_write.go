package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// TwoStageRequested reports whether a call is asking for the plan/apply flow.
//
// It is the gate a generated mutation runs its staged branch behind, and it
// answers the same two questions runTwoStageBranch answers first, in the same
// order: a permitted yolo dominates, and a mode naming neither stage is an
// ordinary call. Asking here rather than inside the flow is what keeps the
// branch's argument checks off an ordinary call, whose contract is that confirm
// gates before anything says whether the arguments would have been accepted.
func TwoStageRequested(ctx context.Context, request *mcp.CallToolRequest) bool {
	if yoloAllowedFromContext(ctx) {
		return false
	}

	mode := request.GetString(paramMode, "")

	return mode == twostage.ModePlan || mode == twostage.ModeApply
}

// RunTwoStageWrite runs the plan and apply branches for a mutation that sends a
// body, and reports handled=false to let the caller fall through to its
// single-step path.
//
// It is the destroy flow's own branch, reached from a second tier rather than
// copied: the plan store, the drift hash, the refusals and the single-use apply
// are one order both tiers share, and the parts a delete does not have are
// already parameters. Which capability the opt-in gate consults is one of them,
// so a Write action stays opt-in by config while every delete keeps opting in by
// default.
//
// The action carries no ConfirmMessage because nothing here reads one: a plan
// reports the change without making it, and an apply is the confirmation.
func RunTwoStageWrite(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, bool) {
	return runTwoStageBranch(ctx, request, cfg, action)
}
