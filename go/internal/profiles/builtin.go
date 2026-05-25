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
// its name. Categorization is prefix-based per the cross-language spec.
//
// A tool may belong to multiple categories (e.g. linode_instance_boot is in
// both "compute" and "compute_actions"). The spec allows collapsing compute,
// compute_actions, and compute_deep into a single "compute" bucket; we keep
// them split because some profiles (storage-admin, kubernetes-admin) lift
// only the deep slice and not the full compute surface.
//
// Returns an empty slice for tools whose name matches no known prefix.
// Phase 8.2 builder tools surface this list via linode_profile_list_tools
// so the model can filter the catalog by category.
func Categories(toolName string) []string {
	cats := make([]string, 0, 2)

	// Core: a small explicit list of meta/account names.
	switch toolName {
	case "hello", "version", "linode_profile", "linode_account", "linode_betas", "linode_beta_get", "linode_account_betas", "linode_account_oauth_clients", "linode_account_payment_methods", "linode_account_payment_method_get", "linode_account_payment_method_create", "linode_account_payment_method_delete", "linode_account_payment_method_make_default", "linode_account_notifications", "linode_account_events", "linode_account_users", "linode_account_user_get", "linode_account_user_grants", "linode_account_user_grants_update", "linode_account_user_update", "linode_account_user_delete", "linode_account_user_create", "linode_account_invoices", "linode_account_payments", "linode_account_payment_create", "linode_account_promo_credit", "linode_account_invoice_items", "linode_account_beta_get":
		cats = append(cats, "core")
	}

	// compute_deep: per-instance backups, stats, disks, and IPs.
	if hasAnyPrefix(
		toolName,
		"linode_instance_backup_",
		"linode_instance_stats_",
		"linode_instance_disk_",
		"linode_instance_ip_",
	) {
		cats = append(cats, "compute_deep")
	}

	// compute_actions: instance lifecycle that's not a deep sub-resource.
	if isComputeAction(toolName) {
		cats = append(cats, "compute_actions")
	}

	// compute: broad compute surface (instances, regions, types, images,
	// stackscripts). Sub-resources under linode_instance_ that already
	// matched compute_deep are still in compute too; profile rules use
	// the union of categories, so duplication is harmless. Tool names are
	// singular across verbs (linode_image_list, linode_stackscript_create),
	// so a single singular prefix per resource covers list/get/write tools.
	if hasAnyPrefix(
		toolName,
		"linode_instance_",
		"linode_region_",
		"linode_type_",
		"linode_image_",
		"linode_stackscript_",
	) {
		cats = append(cats, "compute")
	}

	if strings.HasPrefix(toolName, "linode_volume_") {
		cats = append(cats, "block_storage")
	}

	if strings.HasPrefix(toolName, "linode_object_storage_") {
		cats = append(cats, "object_storage")
	}

	if strings.HasPrefix(toolName, "linode_domain_") {
		cats = append(cats, "dns")
	}

	if hasAnyPrefix(
		toolName,
		"linode_firewall_",
		"linode_nodebalancer_",
		"linode_vlan_",
		"linode_ipv6_range_",
	) {
		cats = append(cats, "networking")
	}

	if strings.HasPrefix(toolName, "linode_database_") {
		cats = append(cats, "databases")
	}

	if strings.HasPrefix(toolName, "linode_lke_") {
		cats = append(cats, "lke")
	}

	if strings.HasPrefix(toolName, "linode_vpc_") {
		cats = append(cats, "vpcs")
	}

	if strings.HasPrefix(toolName, "linode_sshkey_") {
		cats = append(cats, "security")
	}

	if strings.HasPrefix(toolName, "linode_monitor_") {
		cats = append(cats, "monitor")
	}

	return cats
}

// isComputeAction reports whether the tool is one of the instance-lifecycle
// actions that profiles may want to elevate independently of the deep
// sub-resources. Listed explicitly to avoid sweeping up unrelated names that
// happen to share a verb suffix.
func isComputeAction(toolName string) bool {
	switch toolName {
	case "linode_instance_boot",
		"linode_instance_reboot",
		"linode_instance_shutdown",
		"linode_instance_clone",
		"linode_instance_migrate",
		"linode_instance_rebuild",
		"linode_instance_rescue",
		"linode_instance_password_reset",
		"linode_instance_create",
		"linode_instance_delete",
		"linode_instance_resize",
		"linode_instance_get",
		"linode_instance_list":
		return true
	default:
		return false
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
		add("compute", "compute_actions", "compute_deep", "block_storage", "security")
	case BuiltinNetworkAdmin:
		add("networking", "dns", "vpcs")
	case BuiltinKubernetesAdmin:
		add("lke", "compute", "compute_actions", "compute_deep", "vpcs")
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
// catalog and elevated-category set. Rule per spec:
//
//	(Read | Meta) ∪ ((Write | Destroy) ∩ elevated)
//
// Admin capability is excluded from every built-in (no tool carries it
// today). CapUnknown is excluded so a forgotten capability tag cannot leak
// the tool into a profile by accident.
func selectAllowed(catalog []ToolDescriptor, elevated map[string]struct{}) []string {
	allowed := make([]string, 0, len(catalog))

	for _, descriptor := range catalog {
		switch descriptor.Capability {
		case CapRead, CapMeta:
			allowed = append(allowed, descriptor.Name)
		case CapWrite, CapDestroy:
			if isElevated(Categories(descriptor.Name), elevated) {
				allowed = append(allowed, descriptor.Name)
			}
		case CapAdmin, CapUnknown:
			// excluded from every built-in
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
		profiles[name] = p
	}

	return profiles
}

// jsonEntry mirrors Profile's field shape with explicit JSON tags so the
// catalog export is deterministic and snake_cased. Field order, names, and
// types match Profile exactly, which lets BuiltinCatalogJSON convert a
// Profile value into a jsonEntry without copying field-by-field.
type jsonEntry struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	AllowedTools        []string `json:"allowed_tools"`
	AllowedEnvironments []string `json:"allowed_environments"`
	RequiredTokenScopes []string `json:"required_token_scopes"`
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
