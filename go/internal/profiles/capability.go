// Package profiles defines the capability tagging system and (in later
// phases) the user-defined profile registry that controls which tools the
// MCP server exposes to a connected AI client.
//
// Phase 1 of the profiles work introduces only the Capability tag plus a
// temporary allowlist scaffold so the per-category PRs can land in any
// order without breaking the registration invariant tests. The built-in
// profile catalog, config schema, registration filter, hot-reload, and
// token-scope validation arrive in later phases per the profiles spec.
package profiles

import (
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Capability classifies what a tool is allowed to do. The zero value
// CapUnknown is intentional: it marks a tool that has not yet been
// tagged with a real capability. The capability-and-confirm invariant
// check ignores CapUnknown entries during the Phase 1 rollout; the
// Phase 1 cleanup PR adds a stricter assertion that fails on any
// remaining CapUnknown registration.
type Capability int

// The numbers below are the ToolCapability member numbers the contract
// declares, which is what lets String read each tier's name off the generated
// enum instead of keeping a second copy of the names. Renumbering a tag here
// without renumbering the contract spells every later tier as its neighbor,
// so testdata/profile/capability_spellings.json pins the pairing in every
// language.
const (
	// CapUnknown is the zero value. Tools registered without an explicit
	// capability fall here during the Phase 1 rollout. After the Phase 1
	// cleanup PR lands, any tool registering with CapUnknown is a bug.
	CapUnknown Capability = iota
	// CapRead identifies GET endpoints with no state change.
	CapRead
	// CapWrite identifies POST/PUT operations that create or update
	// resources (instance create, firewall update, etc.).
	CapWrite
	// CapDestroy identifies DELETE endpoints and explicitly destructive
	// POSTs (delete instance, rebuild, password reset).
	CapDestroy
	// CapAdmin identifies account-level mutations (account settings
	// update, payment, user management).
	CapAdmin
	// CapMeta identifies tools that touch local config or session state
	// and never the Linode API (profile builder, audit query, version,
	// hello).
	CapMeta
)

// capabilityPrefix opens every capability tag's spelling, so cutting it leaves
// the short form a catalog filter may name instead.
const capabilityPrefix = "Cap"

// memberPrefix is what the contract puts in front of every ToolCapability
// member name, leaving the tier alone once cut.
const memberPrefix = "TOOL_CAPABILITY_"

// untaggedSpelling is what CapUnknown shows. The contract calls its zero member
// TOOL_CAPABILITY_UNSPECIFIED, so the untagged marker is the one tag whose name
// the enum cannot supply without changing what an operator already reads.
const untaggedSpelling = capabilityPrefix + "Unknown"

// invalidSpelling is what a number outside the tag set shows, naming no tier
// because there is none to name.
const invalidSpelling = "Capability(invalid)"

// String returns the capability name for diagnostics and error messages.
func (c Capability) String() string {
	if c == CapUnknown {
		return untaggedSpelling
	}

	// Each declared member number is widened to compare, rather than the tag
	// being narrowed to index the name map: narrowing lets a value outside the
	// int32 range wrap into a neighbor's slot and borrow its tier.
	for number, member := range linodev1.ToolCapability_name {
		// A member the prefix leaves nothing after names no tier either, so it
		// falls through to the same answer a number outside the set gets.
		tier := strings.TrimPrefix(member, memberPrefix)
		if Capability(number) != c || tier == "" {
			continue
		}

		return capabilityPrefix + tier[:1] + strings.ToLower(tier[1:])
	}

	return invalidSpelling
}
