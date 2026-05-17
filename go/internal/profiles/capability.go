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

// Capability classifies what a tool is allowed to do. The zero value
// CapUnknown is intentional: it marks a tool that has not yet been
// tagged with a real capability. The capability-and-confirm invariant
// check ignores CapUnknown entries during the Phase 1 rollout; the
// Phase 1 cleanup PR adds a stricter assertion that fails on any
// remaining CapUnknown registration.
type Capability int

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

// String returns the capability name for diagnostics and error messages.
func (c Capability) String() string {
	switch c {
	case CapUnknown:
		return "CapUnknown"
	case CapRead:
		return "CapRead"
	case CapWrite:
		return "CapWrite"
	case CapDestroy:
		return "CapDestroy"
	case CapAdmin:
		return "CapAdmin"
	case CapMeta:
		return "CapMeta"
	default:
		return "Capability(invalid)"
	}
}
