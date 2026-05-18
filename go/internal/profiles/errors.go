package profiles

import "errors"

// Sentinel errors for profile resolution.
var (
	// ErrActiveProfileUnknown is returned when the configured active profile
	// is neither a built-in name nor a user-defined entry in Config.Profiles.
	ErrActiveProfileUnknown = errors.New("active profile not found")
	// ErrActiveProfileDisabled is returned when the configured active profile
	// names a built-in that has been disabled via
	// Config.ProfilesBuiltinOverrides. User-defined profiles cannot be
	// disabled today, so this only triggers for built-ins.
	ErrActiveProfileDisabled = errors.New("active profile is disabled")
	// ErrProfileFetchFailed wraps any error returned by GetProfile during
	// Phase 6.4 scope validation. Callers can match with errors.Is to
	// distinguish "couldn't reach Linode" from a scope mismatch (which
	// is reported via the ScopeComparison instead of an error return).
	ErrProfileFetchFailed = errors.New("fetch /profile failed")
	// ErrGrantsFetchFailed wraps any error returned by GetProfileGrants
	// on the OAuth code path. Same use as ErrProfileFetchFailed.
	ErrGrantsFetchFailed = errors.New("fetch /profile/grants failed")
	// ErrTokenNotConfigured is returned from Server.ValidateScopes when
	// the active environment has no Linode token set. The caller
	// (typically main) decides what to do: read-only profiles warn and
	// continue, elevated profiles fail to start.
	ErrTokenNotConfigured = errors.New("active environment has no Linode token configured")
)
