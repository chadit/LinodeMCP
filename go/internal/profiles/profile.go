package profiles

// Profile represents a user- or server-defined set of tools, environments, and
// permissions that the active MCP client may use. Built-in profiles are
// declared in code (see builtin.go) so they cannot be edited via config.
// User-defined profiles arrive from config in later phases and reuse this
// same struct.
//
// Field semantics match the cross-language spec in
// .claude/tmp/builtin_profiles_spec.md so the Python implementation produces
// an identical catalog for the parity test.
type Profile struct {
	// Name is the unique identifier, lower-kebab-case.
	Name string
	// Description is a one-line human-readable summary.
	Description string
	// AllowedTools is the resolved list of tool names this profile permits.
	// Wildcards in user-defined profiles expand to explicit lists at config
	// load; built-in profiles populate the list at construction time from
	// the registered tool catalog.
	AllowedTools []string
	// AllowedEnvironments restricts which Linode environments tools may
	// target. An empty list, or a list whose only entry is "*", allows
	// every configured environment.
	AllowedEnvironments []string
	// RequiredTokenScopes lists Linode API scopes the profile assumes.
	// Phase 2 declares them informationally; Phase 6 enforces them at
	// profile load against the active token.
	RequiredTokenScopes []string
	// AllowYolo opts the profile into the yolo execution path. Built-in
	// profiles other than emergency have this set to false.
	AllowYolo bool
	// Disabled, when true, prevents the profile from being selected as
	// active. full-access and emergency are disabled by default; users
	// re-enable them through profiles_builtin_overrides in config.
	Disabled bool
}
