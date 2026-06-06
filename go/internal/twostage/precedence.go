package twostage

import "github.com/chadit/LinodeMCP/internal/profiles"

// Branch is the execution path the precedence helper resolves a call to. It is
// the keystone of the safety story: every two-stage-aware tool routes through
// Resolve so the flag-precedence logic lives in one place instead of scattered
// across handlers.
type Branch string

const (
	// BranchYolo executes immediately, bypassing preview and confirm. Only
	// reachable when the profile permits yolo.
	BranchYolo Branch = "yolo"

	// BranchApply consumes a plan_id, the caller having already drift-checked.
	BranchApply Branch = "apply"

	// BranchPlan produces a plan_id and state hash without changing state.
	BranchPlan Branch = "plan"

	// BranchDryRun produces a preview without a plan_id and without state change.
	BranchDryRun Branch = "dry_run"

	// BranchSingleStep executes a one-shot write (or a plain read) now.
	BranchSingleStep Branch = "single_step"

	// BranchRefuse rejects the call. Decision.ErrCode carries the reason.
	BranchRefuse Branch = "refuse"
)

// PlanLookup classifies the plan store state for an apply request. The caller
// performs the lookup, expiry test, stored-args comparison, and drift fetch,
// then passes the resulting classification so Resolve stays pure and fully
// testable without I/O.
type PlanLookup string

const (
	// PlanLookupNotApplicable is the zero value, used when the call is not an
	// apply.
	PlanLookupNotApplicable PlanLookup = ""

	// PlanLookupValid means the plan exists, has not expired, its stored args
	// match, and the resource has not drifted.
	PlanLookupValid PlanLookup = "valid"

	// PlanLookupExpired means the plan exists but its TTL elapsed.
	PlanLookupExpired PlanLookup = "expired"

	// PlanLookupUnknown means no plan with that ID exists.
	PlanLookupUnknown PlanLookup = "unknown"

	// PlanLookupArgsMismatch means apply-time args differ from stored args.
	PlanLookupArgsMismatch PlanLookup = "args_mismatch"

	// PlanLookupDrifted means the resource changed since the plan was made.
	PlanLookupDrifted PlanLookup = "drifted"
)

const (
	// ModePlan is the MCP mode parameter value that requests a plan.
	ModePlan = "plan"

	// ModeApply is the MCP mode parameter value that applies a stored plan.
	ModeApply = "apply"
)

// Request is the resolved set of two-stage control flags plus the static facts
// about the tool being called. The caller assembles it from the MCP request,
// the tool's capability tag, the opt-in registry, and the active profile.
type Request struct {
	Mode             string
	PlanID           string
	PlanLookup       PlanLookup
	Capability       profiles.Capability
	TwoStageOptedIn  bool
	ProfileAllowYolo bool
	DryRun           bool
	Confirm          bool
	BypassDryRun     bool
	ConfirmedDryRun  bool
	Yolo             bool
}

// Decision is the resolved branch plus a refusal code. ErrCode is empty unless
// Branch is BranchRefuse.
type Decision struct {
	Branch  Branch
	ErrCode string
}

// Resolve maps a Request to a Decision using the spec's precedence order:
// yolo, then apply, then plan, then dry_run, then single-step confirm, then
// refuse. It performs no I/O and never mutates the plan store; the caller acts
// on the returned branch.
func Resolve(req Request) Decision {
	// Reads and meta tools never enter the two-stage machine; they execute as
	// normal calls regardless of the control flags.
	if req.Capability == profiles.CapRead || req.Capability == profiles.CapMeta {
		return Decision{Branch: BranchSingleStep}
	}

	// Yolo dominates every other flag for a mutator.
	if req.Yolo {
		if req.ProfileAllowYolo {
			return Decision{Branch: BranchYolo}
		}

		return Decision{Branch: BranchRefuse, ErrCode: ErrCodeYoloNotPermitted}
	}

	// Contradictory bypass flags are malformed input on any path.
	if req.BypassDryRun && req.ConfirmedDryRun {
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodeBypassFlagsConflict}
	}

	// Plan and apply are available only to opted-in tools.
	if req.TwoStageOptedIn {
		if decision, handled := resolveTwoStageMode(req); handled {
			return decision
		}
	}

	// A preview that produces no plan_id and no state change.
	if req.DryRun {
		return Decision{Branch: BranchDryRun}
	}

	// A one-shot write. CapDestroy still requires a dry-run assertion.
	if req.Confirm {
		return resolveConfirm(req)
	}

	// A mutator called with no execution intent at all.
	return Decision{Branch: BranchRefuse, ErrCode: ErrCodeMissingConfirm}
}

// resolveTwoStageMode handles the plan and apply branches. The second return
// value reports whether the mode matched; an empty or unrecognized mode lets
// Resolve fall through to the dry-run and confirm paths.
func resolveTwoStageMode(req Request) (Decision, bool) {
	switch req.Mode {
	case ModeApply:
		return resolveApply(req), true
	case ModePlan:
		return Decision{Branch: BranchPlan}, true
	default:
		return Decision{}, false
	}
}

// resolveApply maps the caller-supplied plan classification to a branch.
func resolveApply(req Request) Decision {
	if req.PlanID == "" {
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanNotFound}
	}

	switch req.PlanLookup {
	case PlanLookupValid:
		return Decision{Branch: BranchApply}
	case PlanLookupExpired:
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanExpired}
	case PlanLookupArgsMismatch:
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanArgsMismatch}
	case PlanLookupDrifted:
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanDrift}
	case PlanLookupNotApplicable, PlanLookupUnknown:
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanNotFound}
	default:
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodePlanNotFound}
	}
}

// resolveConfirm handles a confirm-only single-step write. A CapDestroy call
// must also carry confirmed_dry_run or confirm_bypass_dry_run, mirroring the
// dry-run spec's bypass gate; other mutators execute on confirm alone.
func resolveConfirm(req Request) Decision {
	if req.Capability == profiles.CapDestroy && !req.ConfirmedDryRun && !req.BypassDryRun {
		return Decision{Branch: BranchRefuse, ErrCode: ErrCodeMissingConfirm}
	}

	return Decision{Branch: BranchSingleStep}
}
