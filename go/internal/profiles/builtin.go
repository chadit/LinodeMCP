package profiles

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ToolDescriptor is the minimal projection of a registered tool that the
// built-in profile resolver needs: the tool name and its capability tag.
// Callers (e.g. server.New in Phase 4 or the parity test) build this slice
// from their own registry and pass it in. Keeping the type local avoids an
// import cycle with internal/server.
type ToolDescriptor struct {
	Name       string
	Capability Capability
}

// Built-in profile names. Exposed as constants so callers reference them
// without typo risk.
const (
	BuiltinDefault         = "default"
	BuiltinReadonlyFull    = "readonly-full"
	BuiltinComputeAdmin    = "compute-admin"
	BuiltinNetworkAdmin    = "network-admin"
	BuiltinKubernetesAdmin = "kubernetes-admin"
	BuiltinStorageAdmin    = "storage-admin"
	BuiltinFullAccess      = "full-access"
	BuiltinEmergency       = "emergency"

	// allEnvironments is the wildcard marker that means "every configured
	// environment". Stored as a list with this single entry to keep JSON
	// shape stable across languages.
	allEnvironments = "*"
)

// Categories returns the list of category names a tool belongs to based on
// its name. Every matching rule contributes, so a tool sits in each category
// it fits and a profile elevating any of them serves it.
//
// Returns an empty slice for tools whose name matches no known prefix.
// Phase 8.2 builder tools surface this list via linode_profile_list_tools
// so the model can filter the catalog by category.
func Categories(toolName string) []string {
	if isCoreTool(toolName) {
		return []string{"core"}
	}

	cats := make([]string, 0, 2)

	for _, rule := range categoryTable() {
		if hasAnyPrefix(toolName, rule.prefixes...) {
			cats = append(cats, rule.category)
		}
	}

	return cats
}

// isCoreTool reports whether a tool is one of the four the "core" category
// holds. Core is the session's own starting point rather than a slice of the
// Linode surface, so it stands apart from the prefix table and no profile
// elevates it; account-gated tools live in "account", which one can.
func isCoreTool(toolName string) bool {
	switch toolName {
	case "hello", "version", "linode_profile_get", "linode_account_get":
		return true
	default:
		return false
	}
}

// categoryTable returns the prefix-to-category rules. This is the same table
// python/src/linodemcp/profiles/builtin.py declares as _TOOL_CATEGORIES, and
// `make profile-resolution` holds the two to one answer per tool: a category a
// tool has in one language and not the other is a profile that serves
// different tools depending on which client the caller runs.
//
// Built fresh per call so the slice doesn't sit as a package-level global,
// matching scopePrefixTable.
func categoryTable() []prefixRule {
	return []prefixRule{
		// Per-instance sub-resources, listed ahead of compute for reading
		// order: storage-admin elevates this slice without the rest of compute.
		{
			category: "compute_deep",
			prefixes: []string{
				"linode_instance_backup_",
				"linode_instance_backups_",
				"linode_instance_disk_",
				"linode_instance_ip_",
				"linode_instance_stats_",
				"linode_instance_transfer_",
			},
		},
		// Tool names are singular across verbs (linode_image_list,
		// linode_stackscript_create), so one prefix per resource covers a
		// family's reads and writes alike.
		{
			category: "compute",
			prefixes: []string{
				"linode_image_",
				"linode_instance_",
				"linode_kernel_",
				"linode_placement_group_",
				"linode_region_",
				"linode_stackscript_",
				"linode_type_",
			},
		},
		// What the API gates on account:*, following scope.go's account rule.
		// The profile prefixes are enumerated rather than swept up under one
		// linode_profile_ entry because the builder's own draft tools share
		// that prefix and never reach the API.
		{
			category: "account",
			prefixes: []string{
				"linode_account_",
				"linode_beta_",
				"linode_lock_",
				"linode_maintenance_policy_",
				"linode_managed_",
				"linode_profile_app_",
				"linode_profile_device_",
				"linode_profile_grants_",
				"linode_profile_login_",
				"linode_profile_phone_number_",
				"linode_profile_preferences_",
				"linode_profile_security_",
				"linode_profile_tfa_",
				"linode_profile_token_",
				"linode_profile_update",
				"linode_support_ticket_",
				"linode_tag_",
			},
		},
		{category: "block_storage", prefixes: []string{"linode_volume_"}},
		{category: "databases", prefixes: []string{"linode_database_"}},
		{category: "object_storage", prefixes: []string{"linode_object_storage_"}},
		{category: "dns", prefixes: []string{"linode_domain_"}},
		{
			category: "networking",
			prefixes: []string{
				"linode_firewall_",
				"linode_ipv6_",
				"linode_network_transfer_",
				"linode_networking_",
				"linode_nodebalancer_",
				"linode_vlan_",
			},
		},
		{category: "lke", prefixes: []string{"linode_lke_"}},
		{category: "vpcs", prefixes: []string{"linode_vpc_"}},
		{category: "security", prefixes: []string{"linode_sshkey_"}},
		{category: "monitor", prefixes: []string{"linode_monitor_"}},
		// Longview carries its own longview:* scope, so a profile that
		// elevates monitor must not reach it.
		{category: "longview", prefixes: []string{"linode_longview_"}},
		{category: "iam", prefixes: []string{"linode_iam_"}},
	}
}

// hasAnyPrefix reports whether toolName starts with any of the given
// prefixes. Helper to keep Categories readable.
func hasAnyPrefix(toolName string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(toolName, p) {
			return true
		}
	}

	return false
}

// elevatedCategories returns the set of categories whose Write/Destroy tools
// the given profile permits. Empty for read-only profiles. The full-access
// and emergency profiles use the sentinel "*" entry meaning "every
// category"; selectAllowed treats that as a wildcard.
func elevatedCategories(profileName string) map[string]struct{} {
	cats := map[string]struct{}{}

	add := func(names ...string) {
		for _, n := range names {
			cats[n] = struct{}{}
		}
	}

	switch profileName {
	case BuiltinDefault, BuiltinReadonlyFull:
		// no elevation
	case BuiltinComputeAdmin:
		add("compute", "compute_deep", "block_storage", "security")
	case BuiltinNetworkAdmin:
		add("networking", "dns", "vpcs")
	case BuiltinKubernetesAdmin:
		add("lke", "compute", "compute_deep", "vpcs")
	case BuiltinStorageAdmin:
		add("block_storage", "object_storage", "compute_deep")
	case BuiltinFullAccess, BuiltinEmergency:
		add(allEnvironments)
	}

	return cats
}

// isElevated reports whether a tool's categories intersect the profile's
// elevated set, or the profile has the "*" wildcard.
func isElevated(toolCats []string, elevated map[string]struct{}) bool {
	if _, all := elevated[allEnvironments]; all {
		return true
	}

	for _, c := range toolCats {
		if _, ok := elevated[c]; ok {
			return true
		}
	}

	return false
}

// selectAllowed resolves the AllowedTools list for a profile from the given
// catalog and elevated-category set. Rule:
//
//	(Read | Meta) ∪ ((Write | Destroy) ∩ elevated) ∪ (Admin ∩ wildcard)
//
// Admin tools are the operations the Linode API gates on the account or
// child_account superscope (account administration, profile token/TFA/
// security/phone self-service, the Managed surface). They are included only
// in the wildcard profiles (full-access and emergency), which carry the
// allEnvironments "*" elevated entry; no category-specific admin profile
// grants them. CapUnknown is excluded so a forgotten capability tag cannot
// leak the tool into a profile by accident.
func selectAllowed(catalog []ToolDescriptor, elevated map[string]struct{}) []string {
	allowed := make([]string, 0, len(catalog))

	_, wildcard := elevated[allEnvironments]

	for _, descriptor := range catalog {
		switch descriptor.Capability {
		case CapRead, CapMeta:
			allowed = append(allowed, descriptor.Name)
		case CapWrite, CapDestroy:
			if isElevated(Categories(descriptor.Name), elevated) {
				allowed = append(allowed, descriptor.Name)
			}
		case CapAdmin:
			if wildcard {
				allowed = append(allowed, descriptor.Name)
			}
		case CapUnknown:
			// excluded so a forgotten capability tag cannot leak the tool
		}
	}

	sort.Strings(allowed)

	return allowed
}

// computeRequiredScopes returns the deduplicated union of RequiredScopes
// over allowedTools, looked up against the catalog for capability. Output
// is sorted ascending so cross-language parity tests stay stable. Tools
// the catalog doesn't know about contribute nothing (matches the
// best-effort fallback in RequiredScopes itself).
func computeRequiredScopes(catalog []ToolDescriptor, allowedTools []string) []string {
	capByName := make(map[string]Capability, len(catalog))
	for _, d := range catalog {
		capByName[d.Name] = d.Capability
	}

	seen := make(map[string]struct{}, len(allowedTools))

	for _, name := range allowedTools {
		capability, ok := capByName[name]
		if !ok {
			continue
		}

		for _, scope := range RequiredScopes(name, capability) {
			seen[string(scope)] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}

	sort.Strings(out)

	return out
}

// BuiltinProfiles returns the catalog of built-in profiles, resolved against
// the supplied tool descriptors. The function is pure: repeated calls with
// the same catalog return equal output. The caller owns the catalog slice
// (typically Server.ToolInfos in production, or a synthetic fixture in
// tests).
//
// The returned map is keyed by profile Name. Each Profile's AllowedTools is
// sorted ascending so cross-language comparisons are stable.
func BuiltinProfiles(catalog []ToolDescriptor) map[string]Profile {
	profiles := map[string]Profile{
		BuiltinDefault: {
			Name:                BuiltinDefault,
			Description:         "Safe read-only default profile. Cannot execute writes, destroys, or admin operations.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinReadonlyFull: {
			Name:                BuiltinReadonlyFull,
			Description:         "Explicit read-only profile spanning every category.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinComputeAdmin: {
			Name:                BuiltinComputeAdmin,
			Description:         "Read everywhere plus write/destroy on compute, block storage, and SSH keys.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinNetworkAdmin: {
			Name:                BuiltinNetworkAdmin,
			Description:         "Read everywhere plus write/destroy on networking, DNS, and VPCs.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinKubernetesAdmin: {
			Name:                BuiltinKubernetesAdmin,
			Description:         "Read everywhere plus write/destroy on LKE, compute, and VPCs.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinStorageAdmin: {
			Name:                BuiltinStorageAdmin,
			Description:         "Read everywhere plus write/destroy on object storage, block storage, and backups.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            false,
		},
		BuiltinFullAccess: {
			Name:                BuiltinFullAccess,
			Description:         "Read, write, and destroy across every category. Disabled by default.",
			AllowedEnvironments: []string{},
			AllowYolo:           false,
			Disabled:            true,
		},
		BuiltinEmergency: {
			Name:                BuiltinEmergency,
			Description:         "Break-glass profile: full access plus yolo execution. Disabled by default.",
			AllowedEnvironments: []string{},
			AllowYolo:           true,
			Disabled:            true,
		},
	}

	// Phase 6.3: RequiredTokenScopes is derived from the resolved tool
	// list rather than hardcoded. Each profile's scope union comes from
	// RequiredScopes(toolName, capability) over its AllowedTools, so a
	// new tool added to a category automatically extends the scope
	// requirement. The previous hardcoded values had Linode-name typos
	// (firewalls plural, ssh_keys, vpcs plural); deriving them fixes
	// the drift in one place.
	for name, p := range profiles {
		p.AllowedTools = selectAllowed(catalog, elevatedCategories(name))
		p.RequiredTokenScopes = computeRequiredScopes(catalog, p.AllowedTools)
		p.Elevated = hasMutatingTools(catalog, p.AllowedTools)
		profiles[name] = p
	}

	return profiles
}

// hasMutatingTools reports whether any allowed tool carries a Write,
// Destroy, or Admin capability per the catalog. This drives the
// Elevated flag; scope suffixes cannot, because the API documents
// write scopes on several read-only routes. Tools the catalog doesn't
// know about contribute nothing, matching computeRequiredScopes.
func hasMutatingTools(catalog []ToolDescriptor, allowedTools []string) bool {
	capByName := make(map[string]Capability, len(catalog))
	for _, d := range catalog {
		capByName[d.Name] = d.Capability
	}

	for _, name := range allowedTools {
		switch capByName[name] {
		case CapWrite, CapDestroy, CapAdmin:
			return true
		case CapRead, CapMeta, CapUnknown:
		}
	}

	return false
}

// jsonEntry mirrors Profile's field shape with explicit JSON tags so the
// catalog export is reproducible and snake_cased. Field order, names, and
// types match Profile exactly, which lets BuiltinCatalogJSON convert a
// Profile value into a jsonEntry without copying field-by-field.
type jsonEntry struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	AllowedTools        []string `json:"allowed_tools"`
	AllowedEnvironments []string `json:"allowed_environments"`
	RequiredTokenScopes []string `json:"required_token_scopes"`
	Elevated            bool     `json:"elevated"`
	AllowYolo           bool     `json:"allow_yolo"`
	Disabled            bool     `json:"disabled"`
}

// BuiltinCatalogJSON returns canonical JSON for the built-in profile catalog
// resolved against the supplied tool descriptors. The output is a list of
// profile objects sorted by name; each profile's AllowedTools is sorted
// ascending so the cross-language parity check can compare byte-for-byte
// against the Python implementation.
func BuiltinCatalogJSON(catalog []ToolDescriptor) ([]byte, error) {
	profiles := BuiltinProfiles(catalog)

	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}

	sort.Strings(names)

	out := make([]jsonEntry, 0, len(names))
	for _, name := range names {
		out = append(out, jsonEntry(profiles[name]))
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal builtin profile catalog: %w", err)
	}

	return data, nil
}
