package profiles

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

	ScopeMonitorReadOnly  Scope = "monitor:read_only"
	ScopeMonitorReadWrite Scope = "monitor:read_write"

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
	categoryEvents        = "events"
	categoryFirewall      = "firewall"
	categoryImages        = "images"
	categoryIPs           = "ips"
	categoryLinodes       = "linodes"
	categoryLKE           = "lke"
	categoryLongview      = "longview"
	categoryMonitor       = "monitor"
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

	// Some tools span more than one category. Attaching a volume hits
	// /volumes but also touches the target Linode, so the API documents
	// linodes:read_write alongside volumes:read_write.
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
	case "linode_account_event_get", "linode_account_event_list":
		// Event routes live under /account but the API gates them with
		// events:* scopes, not account:*. The seen-marker tool is an
		// override instead: the API wants events:read_only on a POST.
		return categoryEvents
	}

	for _, rule := range scopePrefixTable() {
		if hasAnyPrefix(toolName, rule.prefixes...) {
			return rule.category
		}
	}

	return ""
}

// prefixRule pairs tool-name prefixes with the scope category they
// resolve to.
type prefixRule struct {
	prefixes []string
	category string
}

// scopePrefixTable returns the prefix-to-category dispatch rules,
// mirroring Python's _prefix_table. Built fresh per call so the slice
// doesn't sit as a package-level global. Order matters: more specific
// prefixes come first so they are never shadowed by a broader rule.
func scopePrefixTable() []prefixRule {
	return []prefixRule{
		// Managed services, support tickets, tags, and the whole
		// profile subtree are account-gated in the API docs. That
		// includes /profile/tokens, which the docs gate with account:*
		// rather than a dedicated tokens scope. SSH keys live under
		// /profile too.
		{
			prefixes: []string{
				"linode_account_", "linode_managed_", "linode_tag_",
				"linode_support_ticket_", "linode_profile_", "linode_sshkey_",
			},
			category: categoryAccount,
		},
		{prefixes: []string{"linode_database_"}, category: categoryDatabases},
		{prefixes: []string{"linode_object_storage_"}, category: categoryObjectStorage},
		{prefixes: []string{"linode_lke_"}, category: categoryLKE},
		{prefixes: []string{"linode_longview_"}, category: categoryLongview},
		// The /monitor routes carry their own monitor:* scopes.
		{prefixes: []string{"linode_monitor_"}, category: categoryMonitor},
		{prefixes: []string{"linode_nodebalancer_"}, category: categoryNodeBalancers},
		{prefixes: []string{"linode_firewall_"}, category: categoryFirewall},
		{prefixes: []string{"linode_domain_"}, category: categoryDomains},
		{prefixes: []string{"linode_volume_"}, category: categoryVolumes},
		{prefixes: []string{"linode_stackscript_"}, category: categoryStackScripts},
		{prefixes: []string{"linode_vpc_"}, category: categoryVPC},
		{prefixes: []string{"linode_image_"}, category: categoryImages},
		// The /networking/ips, /networking/ipv4, and /networking/ipv6
		// routes all sit on ips:* scopes. The reserved-ip tools never
		// reach here: their explicit cases in scopeCategory win first.
		{
			prefixes: []string{
				"linode_networking_ip_", "linode_networking_ipv4_", "linode_ipv6_",
			},
			category: categoryIPs,
		},
		// VLANs live under /networking/vlans but the API gates them
		// with linodes:* scopes.
		{
			prefixes: []string{
				"linode_instance_", "linode_placement_group_",
				"linode_region_", "linode_type_", "linode_vlan_",
			},
			category: categoryLinodes,
		},
	}
}

// isScopelessRoute reports whether a tool's underlying route is
// documented with no OAuth scope requirement. Two flavors collapse to
// the same empty return: public routes with no security requirement at
// all, and token-only routes documented with an empty scope list,
// meaning any authenticated token may call them. Listing every such
// tool explicitly separates "documented as scopeless" from "forgot to
// map", which the scope completeness test relies on. The scheduled
// sync-scopes gate keeps this list honest against the live spec.
func isScopelessRoute(toolName string) bool {
	switch toolName {
	// Catalog and pricing routes: kernels, regions, instance types,
	// per-service type/price lists, database engines and types.
	case "linode_kernel_get", "linode_kernel_list",
		"linode_region_get", "linode_region_list",
		"linode_region_availability_get", "linode_region_availability_list",
		"linode_type_get", "linode_type_list",
		"linode_database_engine_get", "linode_database_engine_list",
		"linode_database_type_get", "linode_database_type_list",
		"linode_lke_type_list", "linode_longview_type_list",
		"linode_nodebalancer_type_list", "linode_object_storage_type_list",
		"linode_volume_type_list", "linode_network_transfer_price_list":
		return true
	// Token-only or otherwise scopeless per the spec: betas,
	// maintenance, the caller's own profile, Longview subscription
	// plans, VPC reads, the OAuth-client thumbnail, and the metrics
	// query endpoint.
	case "linode_beta_get", "linode_beta_list",
		"linode_maintenance_policy_list", "linode_account_maintenance_list",
		"linode_profile_get",
		"linode_longview_subscription_get", "linode_longview_subscription_list",
		"linode_vpc_get", "linode_vpc_list",
		"linode_vpc_subnet_get", "linode_vpc_subnet_list",
		"linode_account_oauth_client_thumbnail_get",
		"linode_monitor_service_metric_query":
		return true
	}

	return false
}

// scopeOverrides pins tools whose documented OAuth scope cannot be
// derived from the category matrix. Every entry mirrors the security
// block of the underlying operation in the Linode OpenAPI spec, which
// splits these routes away from the rest of their family: reads that
// demand a write scope, writes that only need a read scope, and routes
// gated by a neighboring category. Because several reads here carry
// :read_write, the missing-token elevation policy derives from tool
// capabilities, not scope suffixes (see ProfileIsElevated).
//
// Documented scopes that CANNOT be encoded (typos like ips:read, and
// names absent from every grantable-scope registry like
// placement:read_only) are handled as pinned upstream fixups inside
// scripts/verify_sync_scopes.py, not here; those tools stay on their
// family derivation.
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
		// Reads the API gates behind a write scope: managed contacts
		// hold PII, kubeconfig grants cluster admin, and the instance
		// interface and NodeBalancer node reads are documented that way
		// upstream.
		"linode_managed_contact_get":          {ScopeAccountReadWrite},
		"linode_instance_interface_get":       {ScopeLinodesReadWrite},
		"linode_instance_interface_list":      {ScopeLinodesReadWrite},
		"linode_lke_kubeconfig_get":           {ScopeLKEReadWrite},
		"linode_lke_node_get":                 {ScopeLKEReadWrite},
		"linode_nodebalancer_config_node_get": {ScopeNodeBalancersReadWrite},
		// The docs put this instance-interface read under the
		// NodeBalancers scope; encoded as documented.
		"linode_instance_interface_firewall_list": {ScopeNodeBalancersReadOnly},
		// Writes the API documents with only a read scope.
		"linode_account_event_seen":            {ScopeEventsReadOnly},
		"linode_account_payment_method_delete": {ScopeAccountReadOnly},
		"linode_account_promo_credit_add":      {ScopeAccountReadOnly},
		"linode_monitor_service_token_create":  {ScopeMonitorReadOnly},
		// Presigned URLs can mint upload/delete capability, so the API
		// wants the write scope even though the tool registers as read.
		"linode_object_storage_presigned_url_create": {ScopeObjectStorageReadWrite},
		// VPC IP listings are documented under ips:*, not vpc:*.
		"linode_vpc_ip_list":     {ScopeIPsReadOnly},
		"linode_vpc_ip_all_list": {ScopeIPsReadOnly},
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
		categoryEvents:        {ScopeEventsReadOnly, ScopeEventsReadWrite},
		categoryFirewall:      {ScopeFirewallReadOnly, ScopeFirewallReadWrite},
		categoryImages:        {ScopeImagesReadOnly, ScopeImagesReadWrite},
		categoryIPs:           {ScopeIPsReadOnly, ScopeIPsReadWrite},
		categoryLinodes:       {ScopeLinodesReadOnly, ScopeLinodesReadWrite},
		categoryLKE:           {ScopeLKEReadOnly, ScopeLKEReadWrite},
		categoryLongview:      {ScopeLongviewReadOnly, ScopeLongviewReadWrite},
		categoryMonitor:       {ScopeMonitorReadOnly, ScopeMonitorReadWrite},
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
	case "linode_image_create":
		// Capturing an image reads the source disk, so the API
		// documents linodes:read_only alongside images:read_write.
		return []Scope{ScopeLinodesReadOnly}
	case "linode_volume_attach", "linode_volume_detach":
		// Attach and detach touch the target Linode, so the API
		// documents linodes:read_write alongside volumes:read_write.
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
