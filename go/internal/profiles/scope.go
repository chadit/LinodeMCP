package profiles

// Scope is a Linode OAuth/PAT scope string. The Linode API documents
// scopes as "<resource>:<permission>" pairs (e.g. "linodes:read_only",
// "volumes:read_write"). Personal access tokens carry their scopes as a
// space-delimited string in the /profile response; OAuth tokens express
// the same information through the structured /profile/grants response.
// Phase 6's loader compares the active profile's required scopes against
// the token's actual scopes and fails (or warns) on mismatch.
type Scope string

// Token-side scope vocabulary: the wildcard PAT scope plus the pairs the
// grants converter in scopecheck.go names. Per-tool required scopes are
// declared in the proto contract and served by the generated registry
// tables, so this catalog only spells the values token parsing compares
// against; unknown scopes flow through as plain strings.
const (
	ScopeWildcard Scope = "*"

	ScopeAccountReadOnly  Scope = "account:read_only"
	ScopeAccountReadWrite Scope = "account:read_write"

	ScopeDatabasesReadOnly  Scope = "databases:read_only"
	ScopeDatabasesReadWrite Scope = "databases:read_write"

	ScopeDomainsReadOnly  Scope = "domains:read_only"
	ScopeDomainsReadWrite Scope = "domains:read_write"

	ScopeFirewallReadOnly  Scope = "firewall:read_only"
	ScopeFirewallReadWrite Scope = "firewall:read_write"

	ScopeImagesReadOnly  Scope = "images:read_only"
	ScopeImagesReadWrite Scope = "images:read_write"

	ScopeLinodesReadOnly  Scope = "linodes:read_only"
	ScopeLinodesReadWrite Scope = "linodes:read_write"

	ScopeLKEReadOnly  Scope = "lke:read_only"
	ScopeLKEReadWrite Scope = "lke:read_write"

	ScopeLongviewReadOnly  Scope = "longview:read_only"
	ScopeLongviewReadWrite Scope = "longview:read_write"

	ScopeNodeBalancersReadOnly  Scope = "nodebalancers:read_only"
	ScopeNodeBalancersReadWrite Scope = "nodebalancers:read_write"

	ScopeStackScriptsReadOnly  Scope = "stackscripts:read_only"
	ScopeStackScriptsReadWrite Scope = "stackscripts:read_write"

	ScopeVolumesReadOnly  Scope = "volumes:read_only"
	ScopeVolumesReadWrite Scope = "volumes:read_write"

	ScopeVPCReadOnly  Scope = "vpc:read_only"
	ScopeVPCReadWrite Scope = "vpc:read_write"
)
