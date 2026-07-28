package twostage_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// TestResolveTable is Layer 1 of the precedence-helper test plan: a hand-picked
// table covering every branch, every refusal code, each capability against the
// relevant flag shapes, and both opted-in and opted-out tools.
func TestResolveTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		wantBranch twostage.Branch
		wantErr    string
		req        twostage.Request
	}{
		// Reads and meta short-circuit regardless of flags.
		{"read executes as single step", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapRead}},
		{"read ignores dry_run", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapRead, DryRun: true}},
		{"meta ignores yolo", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapMeta, Yolo: true, ProfileAllowYolo: true}},
		{"read ignores apply", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapRead, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupValid}},

		// Yolo dominates every other flag for a mutator.
		{"yolo permitted on destroy", twostage.BranchYolo, "", twostage.Request{Capability: profiles.CapDestroy, Yolo: true, ProfileAllowYolo: true}},
		{"yolo dominates apply", twostage.BranchYolo, "", twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupValid, Yolo: true, ProfileAllowYolo: true}},
		{"yolo dominates dry_run", twostage.BranchYolo, "", twostage.Request{Capability: profiles.CapWrite, DryRun: true, Yolo: true, ProfileAllowYolo: true}},
		{"yolo not permitted refuses", twostage.BranchRefuse, twostage.ErrCodeYoloNotPermitted, twostage.Request{Capability: profiles.CapDestroy, Yolo: true, ProfileAllowYolo: false}},

		// Contradictory bypass flags are malformed input.
		{"both bypass flags conflict", twostage.BranchRefuse, twostage.ErrCodeBypassFlagsConflict, twostage.Request{Capability: profiles.CapDestroy, Confirm: true, BypassDryRun: true, ConfirmedDryRun: true}},
		{"yolo beats bypass conflict", twostage.BranchYolo, "", twostage.Request{Capability: profiles.CapDestroy, Yolo: true, ProfileAllowYolo: true, BypassDryRun: true, ConfirmedDryRun: true}},

		// Apply branch and its refusals (opted-in tool).
		{"apply valid", twostage.BranchApply, "", twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupValid}},
		{"apply expired", twostage.BranchRefuse, twostage.ErrCodePlanExpired, twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupExpired}},
		{"apply unknown", twostage.BranchRefuse, twostage.ErrCodePlanNotFound, twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupUnknown}},
		{"apply args mismatch", twostage.BranchRefuse, twostage.ErrCodePlanArgsMismatch, twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupArgsMismatch}},
		{"apply drifted", twostage.BranchRefuse, twostage.ErrCodePlanDrift, twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanID: planX, PlanLookup: twostage.PlanLookupDrifted}},
		{"apply with no plan id", twostage.BranchRefuse, twostage.ErrCodePlanNotFound, twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModeApply, PlanLookup: twostage.PlanLookupValid}},

		// Plan branch (opted-in tool).
		{"plan produces a plan", twostage.BranchPlan, "", twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, Mode: twostage.ModePlan}},
		{"admin plan produces a plan", twostage.BranchPlan, "", twostage.Request{Capability: profiles.CapAdmin, TwoStageOptedIn: true, Mode: twostage.ModePlan}},

		// Mode on an opted-out tool falls through to the dry_run and confirm paths.
		{"apply on opted-out falls through to confirm", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapWrite, TwoStageOptedIn: false, Mode: twostage.ModeApply, PlanID: planX, Confirm: true}},
		{"plan on opted-out with no intent refuses", twostage.BranchRefuse, twostage.ErrCodeMissingConfirm, twostage.Request{Capability: profiles.CapWrite, TwoStageOptedIn: false, Mode: twostage.ModePlan}},

		// Dry-run preview.
		{"dry_run on write", twostage.BranchDryRun, "", twostage.Request{Capability: profiles.CapWrite, DryRun: true}},
		{"dry_run on destroy", twostage.BranchDryRun, "", twostage.Request{Capability: profiles.CapDestroy, DryRun: true}},
		{"dry_run on opted-in tool with empty mode", twostage.BranchDryRun, "", twostage.Request{Capability: profiles.CapDestroy, TwoStageOptedIn: true, DryRun: true}},

		// Single-step confirm.
		{"write confirm single step", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapWrite, Confirm: true}},
		{"admin confirm single step", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapAdmin, Confirm: true}},
		{"destroy confirm without assertion refuses", twostage.BranchRefuse, twostage.ErrCodeMissingConfirm, twostage.Request{Capability: profiles.CapDestroy, Confirm: true}},
		{"destroy confirm with confirmed_dry_run", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapDestroy, Confirm: true, ConfirmedDryRun: true}},
		{"destroy confirm with bypass", twostage.BranchSingleStep, "", twostage.Request{Capability: profiles.CapDestroy, Confirm: true, BypassDryRun: true}},

		// No execution intent at all.
		{"write with no intent refuses", twostage.BranchRefuse, twostage.ErrCodeMissingConfirm, twostage.Request{Capability: profiles.CapWrite}},
		{"destroy with no intent refuses", twostage.BranchRefuse, twostage.ErrCodeMissingConfirm, twostage.Request{Capability: profiles.CapDestroy}},
		{"unknown capability with no intent refuses", twostage.BranchRefuse, twostage.ErrCodeMissingConfirm, twostage.Request{Capability: profiles.CapUnknown}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := twostage.Resolve(testCase.req)
			if got.Branch != testCase.wantBranch {
				t.Errorf("branch = %q, want %q", got.Branch, testCase.wantBranch)
			}

			if got.ErrCode != testCase.wantErr {
				t.Errorf("errCode = %q, want %q", got.ErrCode, testCase.wantErr)
			}
		})
	}
}

// expand multiplies a set of partial requests by every value of one field. It
// keeps the cartesian-product builder flat (two loops) instead of deeply
// nested, and the test exhaustively covers the whole input space rather than
// sampling it randomly.
func expand[T any](reqs []twostage.Request, vals []T, set func(*twostage.Request, T)) []twostage.Request {
	out := make([]twostage.Request, 0, len(reqs)*len(vals))

	for _, req := range reqs {
		for _, val := range vals {
			clone := req
			set(&clone, val)
			out = append(out, clone)
		}
	}

	return out
}

// allRequests enumerates every combination of capability, opt-in state, flags,
// mode, plan id presence, and plan-lookup classification: roughly 18k requests.
// Layer 2 of the test plan walks this whole space, which is complete rather
// than sampled.
func allRequests() []twostage.Request {
	bools := []bool{false, true}

	reqs := []twostage.Request{{}}
	reqs = expand(reqs, []profiles.Capability{
		profiles.CapUnknown, profiles.CapRead, profiles.CapWrite,
		profiles.CapDestroy, profiles.CapAdmin, profiles.CapMeta,
	}, func(req *twostage.Request, val profiles.Capability) { req.Capability = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.TwoStageOptedIn = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.ProfileAllowYolo = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.DryRun = val })
	reqs = expand(reqs, []string{"", twostage.ModePlan, twostage.ModeApply, "bogus"},
		func(req *twostage.Request, val string) { req.Mode = val })
	reqs = expand(reqs, []string{"", planX}, func(req *twostage.Request, val string) { req.PlanID = val })
	reqs = expand(reqs, []twostage.PlanLookup{
		twostage.PlanLookupNotApplicable, twostage.PlanLookupValid, twostage.PlanLookupExpired,
		twostage.PlanLookupUnknown, twostage.PlanLookupArgsMismatch, twostage.PlanLookupDrifted,
	}, func(req *twostage.Request, val twostage.PlanLookup) { req.PlanLookup = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.Confirm = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.BypassDryRun = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.ConfirmedDryRun = val })
	reqs = expand(reqs, bools, func(req *twostage.Request, val bool) { req.Yolo = val })

	return reqs
}

func entersMachine(capability profiles.Capability) bool {
	return capability != profiles.CapRead && capability != profiles.CapMeta
}

// TestResolveExhaustiveYoloDominates checks invariant 1 across the whole input
// space: for a mutator, yolo with profile permission always resolves to yolo.
func TestResolveExhaustiveYoloDominates(t *testing.T) {
	t.Parallel()

	for _, base := range allRequests() {
		if !entersMachine(base.Capability) {
			continue
		}

		req := base
		req.Yolo = true
		req.ProfileAllowYolo = true

		if got := twostage.Resolve(req).Branch; got != twostage.BranchYolo {
			t.Fatalf("yolo must dominate for %+v, got branch %q", req, got)
		}
	}
}

// TestResolveExhaustiveRefusalCarriesCode checks invariant 2: a refusal always
// carries an error code and a non-refusal never does.
func TestResolveExhaustiveRefusalCarriesCode(t *testing.T) {
	t.Parallel()

	for _, req := range allRequests() {
		decision := twostage.Resolve(req)

		if (decision.Branch == twostage.BranchRefuse) != (decision.ErrCode != "") {
			t.Fatalf("refusal/code mismatch for %+v: %+v", req, decision)
		}
	}
}

// TestResolveExhaustivePlanApplyPreconditions checks invariant 3: the plan and
// apply branches only occur for an opted-in tool with the matching mode (and,
// for apply, a present, valid plan).
func TestResolveExhaustivePlanApplyPreconditions(t *testing.T) {
	t.Parallel()

	for _, req := range allRequests() {
		switch twostage.Resolve(req).Branch {
		case twostage.BranchApply:
			ok := req.TwoStageOptedIn && req.Mode == twostage.ModeApply &&
				req.PlanID != "" && req.PlanLookup == twostage.PlanLookupValid
			if !ok {
				t.Fatalf("apply branch reached without its preconditions: %+v", req)
			}
		case twostage.BranchPlan:
			if !req.TwoStageOptedIn || req.Mode != twostage.ModePlan {
				t.Fatalf("plan branch reached without its preconditions: %+v", req)
			}
		case twostage.BranchYolo, twostage.BranchDryRun, twostage.BranchSingleStep, twostage.BranchRefuse:
		}
	}
}

// TestResolveExhaustiveCapabilityBoundary checks invariant 4: a read or meta
// call always resolves to single-step execution.
func TestResolveExhaustiveCapabilityBoundary(t *testing.T) {
	t.Parallel()

	for _, req := range allRequests() {
		if entersMachine(req.Capability) {
			continue
		}

		if got := twostage.Resolve(req).Branch; got != twostage.BranchSingleStep {
			t.Fatalf("read/meta must execute as single step, got %q for %+v", got, req)
		}
	}
}

// TestResolveExhaustiveDeterministic checks invariant 5: Resolve is pure, so
// the same request always yields the same decision.
func TestResolveExhaustiveDeterministic(t *testing.T) {
	t.Parallel()

	for _, req := range allRequests() {
		first := twostage.Resolve(req)
		second := twostage.Resolve(req)

		if first != second {
			t.Fatalf("Resolve not reproducible for %+v: %+v vs %+v", req, first, second)
		}
	}
}
