package tools

import "errors"

// Sentinel errors for Linode instance operations.
var (
	ErrEnvironmentNotFound    = errors.New("environment not found in configuration")
	ErrLinodeConfigIncomplete = errors.New("linode configuration is incomplete: check your API URL and token")
	// errUnexpectedKeyToken reports a non-string object key, which valid
	// JSON never produces; it guards the widenObject type assertion.
	errUnexpectedKeyToken = errors.New("unexpected object key token")
)

// Sentinel errors for image share group validation.
var (
	ErrTagsMustBeJSONStringArray = errors.New("tags must be a JSON string array")
	ErrTagsEntriesNonEmpty       = errors.New("tags entries must be non-empty strings")
)

// ErrVLANNotFound is returned when a VLAN dry-run cannot find a matching
// region+label in the VLAN list (VLANs have no single-resource GET).
var ErrVLANNotFound = errors.New("VLAN not found")

// Sentinel errors for bucket validation.
var (
	ErrBucketLabelRequired  = errors.New("label is required")
	ErrBucketRegionRequired = errors.New("region is required")
)

// Sentinel errors for placement group validation.
var (
	ErrPlacementGroupLinodesRequired  = errors.New("linodes is required")
	ErrPlacementGroupLinodesJSON      = errors.New("linodes must be a JSON array of positive integer Linode IDs")
	ErrPlacementGroupLinodesEmpty     = errors.New("linodes must include at least one Linode ID")
	ErrPlacementGroupLinodesPositive  = errors.New("linodes must contain only positive integer Linode IDs")
	ErrPlacementGroupLinodesDuplicate = errors.New("linodes entries must be unique")
)

// The profile-builder tools word their refusals as tool results rather than
// errors, so their sentences live beside the handlers that answer them
// (builderstate.go and linode_profile_draft_save.go) rather than here.
var (
	// ErrNullMember means the response and the declared member were read
	// apart, so there is no object to restore an explicit null into.
	ErrNullMember = errors.New("response carries no object to restore explicit nulls into")
	// ErrResponseDecode wraps the JSON failure when a response object that
	// should hold members does not parse as one.
	ErrResponseDecode = errors.New("failed to decode response object")
	// ErrResponseIndent wraps the JSON failure when a restored response could
	// not be re-indented, which means the restoration built invalid bytes.
	ErrResponseIndent = errors.New("failed to indent proto response")
	// ErrCollectionElement means a removal read the collection its resource
	// belongs to and found no element carrying the id it was addressed by, so
	// there is nothing to preview or hash.
	ErrCollectionElement = errors.New("collection holds no matching element")
	// ErrPageElementCount means a page and the raw bodies it decoded from are
	// different lengths, so element i of one is not element i of the other and
	// restoring by index would write one resource's nulls onto another.
	ErrPageElementCount = errors.New("page element count does not match the raw one")
	// ErrStateNotDeclared means a dependency walk written for a declared fetch
	// was handed some other state, which used to read as a resource carrying
	// nothing and report an empty walk. Python raises the same sentence.
	ErrStateNotDeclared = errors.New("dependency walk received state that is not a declared fetch")
)
