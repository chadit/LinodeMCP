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
)
