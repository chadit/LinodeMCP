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

// Settings carries the operator-tunable two-stage parameters resolved from
// config: the default plan lifetime plus per-tool TTL and opt-in overrides.
// The zero value behaves exactly like the built-in defaults (DefaultPlanTTL
// and the capability-default opt-in), so a caller without config can pass
// Settings{} and get the same answers the package-level helpers give.
type Settings struct {
	// DefaultTTL overrides DefaultPlanTTL for every tool. Non-positive means
	// "unset": fall back to DefaultPlanTTL.
	DefaultTTL time.Duration
	// ToolTTL overrides DefaultTTL for the named tools. A non-positive entry
	// is ignored.
	ToolTTL map[string]time.Duration
	// OptIn forces a tool in (true) or out (false) of the flow by name,
	// overriding the capability default.
	OptIn map[string]bool
}

// OptedIn reports whether a tool participates in the two-stage flow under these
// settings. An explicit OptIn entry wins; otherwise the capability default
// applies: CapDestroy and CapAdmin opt in, everything else stays out.
func (s Settings) OptedIn(tool string, capability profiles.Capability) bool {
	if override, ok := s.OptIn[tool]; ok {
		return override
	}

	return capabilityOptedIn(capability)
}

// PlanTTL returns the plan lifetime for a tool: an explicit per-tool override
// wins, then DefaultTTL, then the built-in DefaultPlanTTL.
func (s Settings) PlanTTL(tool string) time.Duration {
	if ttl, ok := s.ToolTTL[tool]; ok && ttl > 0 {
		return ttl
	}

	if s.DefaultTTL > 0 {
		return s.DefaultTTL
	}

	return DefaultPlanTTL
}

// OptedIn reports whether a tool participates in the two-stage flow under the
// built-in defaults (no config overrides). CapDestroy and CapAdmin default in;
// CapRead, CapWrite, and CapMeta stay out.
func OptedIn(tool string, capability profiles.Capability) bool {
	return Settings{}.OptedIn(tool, capability)
}

// PlanTTL returns the built-in plan lifetime for a tool (DefaultPlanTTL),
// ignoring any config overrides. Callers with config use Settings.PlanTTL.
func PlanTTL(tool string) time.Duration {
	return Settings{}.PlanTTL(tool)
}

// capabilityOptedIn is the capability default shared by Settings.OptedIn and
// the package-level OptedIn: destructive and admin tools opt in, the rest out.
func capabilityOptedIn(capability profiles.Capability) bool {
	switch capability {
	case profiles.CapDestroy, profiles.CapAdmin:
		return true
	case profiles.CapUnknown, profiles.CapRead, profiles.CapWrite, profiles.CapMeta:
		return false
	default:
		return false
	}
}
