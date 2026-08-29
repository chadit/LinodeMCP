package profiles

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
)

// ToolDescriptor is the minimal projection of a registered tool that the
// built-in profile resolver needs. Callers (e.g. server.New in Phase 4 or
// the parity test) build this slice from their own registry and pass it in.
// Keeping the type local avoids an import cycle with internal/server.
type ToolDescriptor struct {
	Name string
	// Scopes is the tool's declared OAuth scope strings, read from the
	// generated registry table at the call sites that build the catalog.
	// Empty for meta tools and documented-scopeless routes.
	Scopes []string
	// Categories is the tool's declared profile categories in declaration
	// order, read from the same generated table. Empty on a declared none.
	Categories []string
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
	BuiltinIamAdmin        = "iam-admin"
	BuiltinFullAccess      = "full-access"
	BuiltinEmergency       = "emergency"

	// allEnvironments is the wildcard marker that means "every configured
	// environment". Stored as a list with this single entry to keep JSON
	// shape stable across languages.
	allEnvironments = "*"
)

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
	case BuiltinIamAdmin:
		// iam only: the legacy account grants tools sit in account, and Linode
		// documents mixing the two access-control systems as a security risk.
		add("iam")
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
			if isElevated(descriptor.Categories, elevated) {
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

// computeRequiredScopes returns the deduplicated union of the catalog's
// declared per-tool scopes over allowedTools. Output is sorted ascending so
// cross-language parity tests stay stable. Tools the catalog doesn't know
// about contribute nothing.
func computeRequiredScopes(catalog []ToolDescriptor, allowedTools []string) []string {
	scopesByName := make(map[string][]string, len(catalog))
	for _, d := range catalog {
		scopesByName[d.Name] = d.Scopes
	}

	seen := make(map[string]struct{}, len(allowedTools))

	for _, name := range allowedTools {
		for _, scope := range scopesByName[name] {
			seen[scope] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}

	sort.Strings(out)

	return out
}

// builtinBlueprints is the nine built-in profiles before a catalog resolves
// their tool lists. One place carries their names and descriptions, so a
// built-in added here reaches every reader of the set.
func builtinBlueprints() map[string]Profile {
	return map[string]Profile{
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
		BuiltinIamAdmin: {
			Name:                BuiltinIamAdmin,
			Description:         "Read everywhere plus write/destroy on identity and access management: role assignment, delegation, and IdP/SSO configs.",
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
}

// BuiltinProfileNames is the names the built-in profiles occupy, sorted. Read
// off the blueprints rather than restated, so a built-in added above is refused
// as a save target without a second list needing the same edit.
func BuiltinProfileNames() []string {
	return slices.Sorted(maps.Keys(builtinBlueprints()))
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
	profiles := builtinBlueprints()

	// Phase 6.3: RequiredTokenScopes is derived from the resolved tool
	// list rather than hardcoded. Each profile's scope union comes from
	// the catalog's declared per-tool scopes over its AllowedTools, so a
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
