package builder

import "errors"

// Sentinel errors for the draft registry. Callers match with errors.Is.
var (
	// ErrDraftNameEmpty is returned by Create when name is empty.
	ErrDraftNameEmpty = errors.New("draft name cannot be empty")
	// ErrDraftExists is returned by Create when the registry already
	// holds a draft with the given name. Discard the existing draft
	// first, or pick a different name. We refuse silent overwrite so
	// a stray reroll doesn't lose work.
	ErrDraftExists = errors.New("draft already exists")
)
