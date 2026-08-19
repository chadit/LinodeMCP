package tools

import (
	"fmt"
)

// Phase 2 dependency-walk shared vocabulary. Action names match the spec's
// dependencies[].action enum; BillingUnknown is the sentinel returned when
// a cost estimate cannot be computed.
const (
	// DependencyActionDetached and the three action names below it are exported
	// because a generated tool's dependency walk lives in internal/toolhooks,
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

// LabelChangeSideEffect appends a "label changes from X to Y" (or "label set
// to Y") side effect when newLabel is non-empty, comparing against the current
// label from the fetched state.
func LabelChangeSideEffect(details *DryRunDetails, fromLabel, newLabel string) {
	if newLabel == "" {
		return
	}

	if fromLabel != "" && fromLabel != newLabel {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Label changes from %q to %q.", fromLabel, newLabel))

		return
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("Label is set to %q.", newLabel))
}

// profileTokenCreateSideEffects is the Tier B preview for
// linode_profile_token_create. It names the token and warns that the secret is
// shown only once (credential-sensitive; arg-only, no fetch).

// profileTFAEnableSideEffects is the Tier B preview for
// linode_profile_tfa_enable. It generates a 2FA secret that must still be
// confirmed before 2FA is active (arg-only, no fetch).

// profilePreferencesUpdateSideEffects was the Tier B preview for
// linode_profile_preferences_update. The tool is generated now, so the replace
// semantics it used to report live in the tool's own description, where every
// caller reads them rather than only the ones who ask for a dry run.

// profileTokenUpdateSideEffects is the Tier B preview for
// linode_profile_token_update. The update tool changes the token's label
// (arg-only, no fetch).

// profilePhoneNumberSendSideEffects is the Tier B preview for
// linode_profile_phone_number_send. The phone number is PII, so the side
// effect avoids echoing it (arg-only).

// profilePhoneNumberVerifySideEffects is the Tier B preview for
// linode_profile_phone_number_verify (arg-only).
