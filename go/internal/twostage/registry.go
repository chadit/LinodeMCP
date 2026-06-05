// Package twostage implements the plan/apply (two-stage write) flow: a
// destructive call first produces a plan with a state hash and an ID, the
// user reviews it, then applies by reference. The server refuses to apply
// whenever the underlying resource drifted between plan and apply.
//
// Phase 1 (this code) builds the foundation: the opt-in registry, the
// in-memory plan store with a TTL janitor and a size ceiling, plan-ID
// generation, and the typed errors the apply path returns. The precedence
// helper and per-tool wiring land in later Phase 1 slices.
//
// The package keeps its lookup tables behind functions rather than package
// vars so they stay read-only and the codebase avoids global mutable state
// (the same convention the audit and dry-run packages use).
package twostage

import (
	"time"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

// DefaultPlanTTL is how long a plan stays valid before it expires. Five
// minutes balances enough review time for a human against the risk that
// state drifts between plan and apply.
const DefaultPlanTTL = 5 * time.Minute

// OptedIn reports whether a tool participates in the two-stage flow. The
// explicit registry overrides the capability default: CapDestroy and CapAdmin
// default in, CapWrite defaults out (flip per tool in the registry), CapRead
// and CapMeta are never opted in.
func OptedIn(tool string, capability profiles.Capability) bool {
	if override, ok := optedInOverrides()[tool]; ok {
		return override
	}

	switch capability {
	case profiles.CapDestroy, profiles.CapAdmin:
		return true
	case profiles.CapUnknown, profiles.CapRead, profiles.CapWrite, profiles.CapMeta:
		return false
	default:
		return false
	}
}

// optedInOverrides is the explicit per-tool registry. Only deviations from the
// capability default belong here: a CapWrite tool that should opt in, or a
// CapDestroy/CapAdmin tool that should opt out. CapDestroy and CapAdmin tools
// need no entry; the capability default in OptedIn already covers them.
// Phase 2 populates this as tools gain state-fetch support.
func optedInOverrides() map[string]bool {
	return map[string]bool{}
}

// PlanTTL returns the plan lifetime for a tool, honoring per-tool overrides
// and falling back to DefaultPlanTTL. Tools with a larger blast radius can be
// given more review time here.
func PlanTTL(tool string) time.Duration {
	if ttl, ok := planTTLOverrides()[tool]; ok {
		return ttl
	}

	return DefaultPlanTTL
}

func planTTLOverrides() map[string]time.Duration {
	return map[string]time.Duration{}
}
