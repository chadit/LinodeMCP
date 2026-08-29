package tools

// Phase 2 dependency-walk shared vocabulary. Action names match the spec's
// dependencies[].action enum; BillingUnknown is the sentinel returned when
// a cost estimate cannot be computed.
const (
	// DependencyActionDetached and the three action names below it are exported
	// because a generated tool's dependency walk is emitted into internal/gentools,
	// and they are part of the preview contract a walk fills rather than each
	// walk's own wording.
	DependencyActionDetached       = "detached"
	DependencyActionReleased       = "released"
	DependencyActionRemoved        = "removed"
	DependencyActionCascadeDeleted = "cascade_deleted"

	// BillingUnknown is the sentinel a walk reports when a cost estimate cannot
	// be computed. Exported for the same reason the action names are.
	BillingUnknown = "unknown"

	// DependencyWalkPageSize bounds a single dependency-list fetch. A
	// resource with more dependents than this is rare; the walk notes a
	// possible truncation rather than paging exhaustively during a preview.
	DependencyWalkPageSize = 100
)
