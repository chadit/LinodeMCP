package profiles

import "strings"

// Scope is a Linode OAuth/PAT scope string. The Linode API documents
// scopes as "<resource>:<permission>" pairs (e.g. "linodes:read_only",
// "volumes:read_write"). Personal access tokens carry their scopes as a
// space-delimited string in the /profile response; OAuth tokens express
// the same information through the structured /profile/grants response.
// Phase 6's loader compares the active profile's required scopes against
// the token's actual scopes and fails (or warns) on mismatch.
type Scope string

// Linode scope catalog. The strings exactly match the values the Linode
// API expects in token scope strings and grant entries. Wildcard `*` is
// the all-access PAT scope; everything else pairs a resource category
// with a read-only or read-write permission. The full list is derived
// from Linode's API documentation as of 2026-05-17; adding new scopes is
// safe and additive since unknown ones flow through as plain strings.
const (
	ScopeWildcard Scope = "*"

	ScopeAccountReadOnly  Scope = "account:read_only"
	ScopeAccountReadWrite Scope = "account:read_write"

	ScopeDatabasesReadOnly  Scope = "databases:read_only"
	ScopeDatabasesReadWrite Scope = "databases:read_write"

	ScopeDomainsReadOnly  Scope = "domains:read_only"
	ScopeDomainsReadWrite Scope = "domains:read_write"

	ScopeEventsReadOnly  Scope = "events:read_only"
	ScopeEventsReadWrite Scope = "events:read_write"

	ScopeFirewallReadOnly  Scope = "firewall:read_only"
	ScopeFirewallReadWrite Scope = "firewall:read_write"

	ScopeImagesReadOnly  Scope = "images:read_only"
	ScopeImagesReadWrite Scope = "images:read_write"

	ScopeIPsReadOnly  Scope = "ips:read_only"
	ScopeIPsReadWrite Scope = "ips:read_write"

	ScopeLinodesReadOnly  Scope = "linodes:read_only"
	ScopeLinodesReadWrite Scope = "linodes:read_write"

	ScopeLKEReadOnly  Scope = "lke:read_only"
	ScopeLKEReadWrite Scope = "lke:read_write"

	ScopeLongviewReadOnly  Scope = "longview:read_only"
	ScopeLongviewReadWrite Scope = "longview:read_write"

	ScopeMaintenanceReadOnly  Scope = "maintenance:read_only"
	ScopeMaintenanceReadWrite Scope = "maintenance:read_write"

	ScopeNodeBalancersReadOnly  Scope = "nodebalancers:read_only"
	ScopeNodeBalancersReadWrite Scope = "nodebalancers:read_write"

	ScopeObjectStorageReadOnly  Scope = "object_storage:read_only"
	ScopeObjectStorageReadWrite Scope = "object_storage:read_write"

	ScopeReservedIPsReadOnly  Scope = "reserved-ips:read_only"
	ScopeReservedIPsReadWrite Scope = "reserved-ips:read_write"

	ScopeStackScriptsReadOnly  Scope = "stackscripts:read_only"
	ScopeStackScriptsReadWrite Scope = "stackscripts:read_write"

	ScopeUsersReadOnly  Scope = "users:read_only"
	ScopeUsersReadWrite Scope = "users:read_write"

	ScopeVolumesReadOnly  Scope = "volumes:read_only"
	ScopeVolumesReadWrite Scope = "volumes:read_write"

	ScopeVPCReadOnly  Scope = "vpc:read_only"
	ScopeVPCReadWrite Scope = "vpc:read_write"
)

// Scope category names. Internal markers used by the prefix mapping;
// they round-trip into the scope strings via scopeFor. Extracting them
// as constants keeps the prefix dispatcher and the scope catalog from
// drifting on the literal spelling.
const (
	categoryAccount       = "account"
	categoryDatabases     = "databases"
	categoryDomains       = "domains"
	categoryFirewall      = "firewall"
	categoryImages        = "images"
	categoryIPs           = "ips"
	categoryLinodes       = "linodes"
	categoryLKE           = "lke"
	categoryLongview      = "longview"
	categoryNodeBalancers = "nodebalancers"
	categoryObjectStorage = "object_storage"
	categoryReservedIPs   = "reserved-ips"
	categoryStackScripts  = "stackscripts"
	categoryVolumes       = "volumes"
	categoryVPC           = "vpc"
)

// RequiredScopes returns the Linode scope(s) a tool needs the active
// token to carry. The mapping is name-prefix based, mirroring how
// Categories() in builtin.go assigns tools to profile categories. The
// capability tells whether the tool reads or writes, which decides
// between :read_only and :read_write.
//
// Meta tools (hello, version) return an empty slice: they touch no
// Linode API. A nil/empty return means "no scope required".
//
// Unknown tool names return nil too. The Phase 6.4 loader treats unknown
// names as "best effort" rather than a hard failure so a forgotten
// mapping degrades gracefully into a logged warning. The scope
// completeness test in internal/server closes the remaining gap: every
// registered tool must resolve to a non-empty scope list or sit on the
// documented scopeless list, so a forgotten mapping fails tests instead
// of shipping as silently unrestricted.
func RequiredScopes(toolName string, capability Capability) []Scope {
	if capability == CapMeta {
		return nil
	}

	if isScopelessRoute(toolName) {
		return nil
	}

	if scopes, ok := scopeOverrides()[toolName]; ok {
		return scopes
	}

	category := scopeCategory(toolName)
	if category == "" {
		return nil
	}

	scope, ok := scopeFor(category, capability)
	if !ok {
		return nil
	}

	// Some tools span more than one category. linode_instance_create
	// hits /linode/instances but also pulls images, so any token that
	// wants to provision an image-deployed instance needs
	// images:read_only too.
	extras := additionalScopes(toolName)
	if len(extras) == 0 {
		return []Scope{scope}
	}

	out := make([]Scope, 0, 1+len(extras))
	out = append(out, scope)
	out = append(out, extras...)

	return out
}

// scopeCategory returns the Linode scope category name (the part before
// the colon in a scope string) for a given tool name, or "" if the tool
// doesn't fit a known category. The match order matters: longest prefix
// wins so linode_instance_backup_* is routed via instance rather than
// being shadowed by a more general linode_instance_ rule.
func scopeCategory(toolName string) string {
	switch toolName {
	case "linode_networking_reserved_ip_list":
		return categoryIPs
	case "linode_networking_reserved_ip_delete":
		return categoryReservedIPs
	}

	// Order matters: longer prefixes first.
	switch {
	case strings.HasPrefix(toolName, "linode_account_"):
		return categoryAccount
	case strings.HasPrefix(toolName, "linode_database_"):
		return categoryDatabases
	case strings.HasPrefix(toolName, "linode_object_storage_"):
		return categoryObjectStorage
	case strings.HasPrefix(toolName, "linode_lke_"):
		return categoryLKE
	case strings.HasPrefix(toolName, "linode_longview_"):
		return categoryLongview
	case strings.HasPrefix(toolName, "linode_nodebalancer_"):
		return categoryNodeBalancers
	case strings.HasPrefix(toolName, "linode_firewall_"):
		return categoryFirewall
	case strings.HasPrefix(toolName, "linode_domain_"):
		return categoryDomains
	case strings.HasPrefix(toolName, "linode_volume_"):
		return categoryVolumes
	case strings.HasPrefix(toolName, "linode_stackscript_"):
		return categoryStackScripts
	case strings.HasPrefix(toolName, "linode_vpc_"):
		return categoryVPC
	case strings.HasPrefix(toolName, "linode_image_"):
		return categoryImages
	case strings.HasPrefix(toolName, "linode_monitor_"),
		strings.HasPrefix(toolName, "linode_sshkey_"):
		// Monitor and SSH-key tools live under account-scoped endpoints.
		return categoryAccount
	case strings.HasPrefix(toolName, "linode_managed_"),
		strings.HasPrefix(toolName, "linode_support_ticket_"),
		strings.HasPrefix(toolName, "linode_tag_"),
		strings.HasPrefix(toolName, "linode_profile_"):
		// Managed services, support tickets, tags, and the whole
		// profile subtree are account-gated in the API docs. That
		// includes /profile/tokens, which the docs gate with account:*
		// rather than a dedicated tokens scope.
		return categoryAccount
	case strings.HasPrefix(toolName, "linode_networking_ip_"),
		strings.HasPrefix(toolName, "linode_networking_ipv4_"),
		strings.HasPrefix(toolName, "linode_ipv6_"):
		// The /networking/ips, /networking/ipv4, and /networking/ipv6
		// routes all sit on ips:* scopes. The reserved-ip tools never
		// reach here: their explicit cases above win first.
		return categoryIPs
	case strings.HasPrefix(toolName, "linode_instance_"),
		strings.HasPrefix(toolName, "linode_placement_group_"),
		strings.HasPrefix(toolName, "linode_region_"),
		strings.HasPrefix(toolName, "linode_type_"),
		strings.HasPrefix(toolName, "linode_vlan_"):
		// VLANs live under /networking/vlans but the API gates them
		// with linodes:* scopes.
		return categoryLinodes
	}

	return ""
}

// isScopelessRoute reports whether a tool's underlying route is
// documented with no OAuth scope requirement. Two flavors collapse to
// the same empty return: public catalog routes that need no
// authentication at all (kernels, database engines and types), and
// token-only routes documented with an empty scope list, meaning any
// authenticated token may call them (betas, maintenance policies).
// Listing them explicitly separates "documented as scopeless" from
// "forgot to map", which the scope completeness test relies on.
func isScopelessRoute(toolName string) bool {
	switch toolName {
	case "linode_kernel_get", "linode_kernel_list",
		"linode_database_engine_get", "linode_database_engine_list",
		"linode_database_type_get", "linode_database_type_list",
		"linode_network_transfer_price_list",
		"linode_beta_get", "linode_beta_list",
		"linode_maintenance_policy_list":
		return true
	}

	return false
}

// scopeOverrides pins tools whose documented OAuth scope cannot be
// derived from the category matrix. Each entry mirrors the security
// block of the underlying operation in the Linode OpenAPI spec, which
// splits these routes away from the rest of their family.
//
// Two documented quirks are deliberately NOT mirrored here.
// GET /managed/contacts/{contactId} documents account:read_write on a
// read: encoding that would make the read-only builtin profiles carry a
// write scope and flip the missing-token elevation policy for everyone
// on the default profile. GET /placement/groups documents
// placement:read_only, a scope the spec's own OAuth catalog never
// defines, so tokens may not be able to carry it at all. Both tools
// stay on their family derivation until the upstream docs and the
// grantable scope set agree.
func scopeOverrides() map[string][]Scope {
	return map[string][]Scope{
		// PUT /networking/firewalls/settings is documented as
		// account:read_write while GET on the same route stays
		// firewall:read_only.
		"linode_firewall_settings_update": {ScopeAccountReadWrite},
		// GET .../instances/{id}/credentials is documented as
		// databases:read_only on both engines even though the tools
		// register as mutators (they expose secrets, so the server
		// gates them behind a stronger capability).
		"linode_database_mysql_instance_credentials_get":      {ScopeDatabasesReadOnly},
		"linode_database_postgresql_instance_credentials_get": {ScopeDatabasesReadOnly},
	}
}

// scopeMatrix is built fresh per call (cheap, 15 entries) so we avoid a
// package-level global the gochecknoglobals linter would flag. The two
// entries per row are [read-only, read-write]. Order matters: index 0
// is read-only.
func scopeMatrix() map[string][2]Scope {
	return map[string][2]Scope{
		categoryAccount:       {ScopeAccountReadOnly, ScopeAccountReadWrite},
		categoryDatabases:     {ScopeDatabasesReadOnly, ScopeDatabasesReadWrite},
		categoryDomains:       {ScopeDomainsReadOnly, ScopeDomainsReadWrite},
		categoryFirewall:      {ScopeFirewallReadOnly, ScopeFirewallReadWrite},
		categoryImages:        {ScopeImagesReadOnly, ScopeImagesReadWrite},
		categoryIPs:           {ScopeIPsReadOnly, ScopeIPsReadWrite},
		categoryLinodes:       {ScopeLinodesReadOnly, ScopeLinodesReadWrite},
		categoryLKE:           {ScopeLKEReadOnly, ScopeLKEReadWrite},
		categoryLongview:      {ScopeLongviewReadOnly, ScopeLongviewReadWrite},
		categoryNodeBalancers: {ScopeNodeBalancersReadOnly, ScopeNodeBalancersReadWrite},
		categoryObjectStorage: {ScopeObjectStorageReadOnly, ScopeObjectStorageReadWrite},
		categoryReservedIPs:   {ScopeReservedIPsReadOnly, ScopeReservedIPsReadWrite},
		categoryStackScripts:  {ScopeStackScriptsReadOnly, ScopeStackScriptsReadWrite},
		categoryVolumes:       {ScopeVolumesReadOnly, ScopeVolumesReadWrite},
		categoryVPC:           {ScopeVPCReadOnly, ScopeVPCReadWrite},
	}
}

// scopeFor maps a category and capability to its Linode scope string.
// Read-only capabilities pair with :read_only; mutators (CapWrite,
// CapDestroy, CapAdmin) pair with :read_write since the Linode API does
// not distinguish further. The bool is false if the category has no
// mapped scope at all.
func scopeFor(category string, capability Capability) (Scope, bool) {
	pair, ok := scopeMatrix()[category]
	if !ok {
		return "", false
	}

	if capability == CapRead {
		return pair[0], true
	}

	return pair[1], true
}

// additionalScopes returns secondary scopes a tool needs beyond its
// primary category. Some Linode endpoints touch multiple resource
// categories (creating an instance from an image needs images:read_only
// alongside linodes:read_write). Most tools have no extras and return
// nil.
func additionalScopes(toolName string) []Scope {
	switch toolName {
	case "linode_instance_create", "linode_instance_clone", "linode_instance_rebuild":
		// Provisioning from an image requires images:read_only.
		return []Scope{ScopeImagesReadOnly}
	case "linode_lke_cluster_create":
		// LKE clusters provision Linodes under the hood.
		return []Scope{ScopeLinodesReadWrite}
	case "linode_networking_ip_assign", "linode_networking_ip_share",
		"linode_networking_ipv4_assign", "linode_networking_ipv4_share",
		"linode_ipv6_range_create":
		// Assigning or sharing addresses targets Linodes, so the API
		// documents linodes:read_write alongside ips:read_write.
		return []Scope{ScopeLinodesReadWrite}
	}

	return nil
}
