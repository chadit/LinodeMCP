package profiles

import (
	"slices"
	"strings"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// ParsePATScopes splits a Linode PAT scope string (the value returned in
// the /profile response's `scopes` field) into a deduplicated Scope set.
// The format is space-delimited tokens; "*" stands for "all permissions"
// on the matching category. Empty/whitespace input yields an empty set.
func ParsePATScopes(scopeStr string) []Scope {
	fields := strings.Fields(scopeStr)
	if len(fields) == 0 {
		return nil
	}

	seen := make(map[Scope]struct{}, len(fields))
	for _, f := range fields {
		seen[Scope(f)] = struct{}{}
	}

	out := make([]Scope, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}

	slices.Sort(out)

	return out
}

// FlattenGrants walks a /profile/grants response and returns the
// effective set of Scope values it grants. Global account booleans
// produce account-level scopes; per-resource grants imply the matching
// category scope. The returned slice is sorted and deduplicated.
//
// The mapping is conservative: a grant present at any permission level
// implies the matching :read_only scope; permission "read_write" also
// implies :read_write. Empty/null permission entries grant nothing.
func FlattenGrants(grants *linode.Grants) []Scope {
	if grants == nil {
		return nil
	}

	seen := make(map[Scope]struct{})
	collectGlobal(grants.Global, seen)
	collectResourceGrants(grants, seen)

	out := make([]Scope, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}

	slices.Sort(out)

	return out
}

// collectGlobal adds account-level scopes for each true bool on
// GlobalGrants. AccountAccess gives the :read_only/:read_write pair
// directly; the add_* flags imply the resource category's :read_write
// (a token that can "add linodes" can write linodes).
func collectGlobal(global linode.GlobalGrants, seen map[Scope]struct{}) {
	switch global.AccountAccess {
	case "read_write":
		seen[ScopeAccountReadWrite] = struct{}{}
		seen[ScopeAccountReadOnly] = struct{}{}
	case "read_only":
		seen[ScopeAccountReadOnly] = struct{}{}
	}

	if global.AddLinodes {
		seen[ScopeLinodesReadWrite] = struct{}{}
		seen[ScopeLinodesReadOnly] = struct{}{}
	}

	if global.AddDomains {
		seen[ScopeDomainsReadWrite] = struct{}{}
		seen[ScopeDomainsReadOnly] = struct{}{}
	}

	if global.AddFirewalls {
		seen[ScopeFirewallReadWrite] = struct{}{}
		seen[ScopeFirewallReadOnly] = struct{}{}
	}

	if global.AddImages {
		seen[ScopeImagesReadWrite] = struct{}{}
		seen[ScopeImagesReadOnly] = struct{}{}
	}

	if global.AddDatabases {
		seen[ScopeDatabasesReadWrite] = struct{}{}
		seen[ScopeDatabasesReadOnly] = struct{}{}
	}

	if global.AddLongview {
		seen[ScopeLongviewReadWrite] = struct{}{}
		seen[ScopeLongviewReadOnly] = struct{}{}
	}

	if global.AddNodeBalancers {
		seen[ScopeNodeBalancersReadWrite] = struct{}{}
		seen[ScopeNodeBalancersReadOnly] = struct{}{}
	}

	if global.AddStackScripts {
		seen[ScopeStackScriptsReadWrite] = struct{}{}
		seen[ScopeStackScriptsReadOnly] = struct{}{}
	}

	if global.AddVolumes {
		seen[ScopeVolumesReadWrite] = struct{}{}
		seen[ScopeVolumesReadOnly] = struct{}{}
	}

	if global.AddVPCs {
		seen[ScopeVPCReadWrite] = struct{}{}
		seen[ScopeVPCReadOnly] = struct{}{}
	}
}

// collectResourceGrants walks the per-resource grant lists and adds the
// matching category scopes for any non-empty permission. A grant with
// permission "read_write" produces both read_only and read_write; a
// grant with "read_only" produces only :read_only.
func collectResourceGrants(grants *linode.Grants, seen map[Scope]struct{}) {
	addPair := func(perm linode.GrantPermission, readOnly, readWrite Scope) {
		switch perm {
		case "read_write":
			seen[readWrite] = struct{}{}
			seen[readOnly] = struct{}{}
		case "read_only":
			seen[readOnly] = struct{}{}
		}
	}

	for _, g := range grants.Linode {
		addPair(g.Permissions, ScopeLinodesReadOnly, ScopeLinodesReadWrite)
	}

	for _, g := range grants.Domain {
		addPair(g.Permissions, ScopeDomainsReadOnly, ScopeDomainsReadWrite)
	}

	for _, g := range grants.NodeBalancer {
		addPair(g.Permissions, ScopeNodeBalancersReadOnly, ScopeNodeBalancersReadWrite)
	}

	for _, g := range grants.Image {
		addPair(g.Permissions, ScopeImagesReadOnly, ScopeImagesReadWrite)
	}

	for _, g := range grants.Longview {
		addPair(g.Permissions, ScopeLongviewReadOnly, ScopeLongviewReadWrite)
	}

	for _, g := range grants.StackScript {
		addPair(g.Permissions, ScopeStackScriptsReadOnly, ScopeStackScriptsReadWrite)
	}

	for _, g := range grants.Volume {
		addPair(g.Permissions, ScopeVolumesReadOnly, ScopeVolumesReadWrite)
	}

	for _, g := range grants.Database {
		addPair(g.Permissions, ScopeDatabasesReadOnly, ScopeDatabasesReadWrite)
	}

	for _, g := range grants.Firewall {
		addPair(g.Permissions, ScopeFirewallReadOnly, ScopeFirewallReadWrite)
	}

	for _, g := range grants.VPC {
		addPair(g.Permissions, ScopeVPCReadOnly, ScopeVPCReadWrite)
	}

	for _, g := range grants.LKECluster {
		addPair(g.Permissions, ScopeLKEReadOnly, ScopeLKEReadWrite)
	}
}

// ScopeComparison captures the result of comparing a profile's required
// scope list against a token's actual scope list. Missing scopes are a
// hard failure (the token can't do what the profile needs). Excess
// scopes are a least-privilege warning by default; strict mode promotes
// them to errors.
type ScopeComparison struct {
	// Missing lists scopes the profile requires but the token lacks.
	// Sorted ascending so error messages stay stable.
	Missing []Scope

	// Excess lists scopes the token carries that the profile does not
	// require. Sorted ascending.
	Excess []Scope
}

// HasMissing reports whether the token is under-scoped for the active
// profile. Phase 6.4 treats this as a hard failure at load time.
func (c ScopeComparison) HasMissing() bool { return len(c.Missing) > 0 }

// HasExcess reports whether the token carries more access than the
// active profile asks for. Warning by default; strict mode treats it
// as a failure.
func (c ScopeComparison) HasExcess() bool { return len(c.Excess) > 0 }

// CompareScopes returns the missing/excess sets between a profile's
// required scopes and the token's actual scopes. Wildcard handling:
//   - ScopeWildcard ("*") in `actual` matches any required scope.
//   - A required scope of "*" is not meaningful (callers should keep
//     this out of profile definitions) but is treated as "no specific
//     scope required" so an empty required slice and a `["*"]` required
//     slice both pass.
//
// Comparison is set-based: order of inputs doesn't matter. Output is
// sorted ascending so error messages and test fixtures stay stable.
func CompareScopes(required, actual []Scope) ScopeComparison {
	actualSet := make(map[Scope]struct{}, len(actual))

	var hasWildcard bool

	for _, s := range actual {
		actualSet[s] = struct{}{}
		if s == ScopeWildcard {
			hasWildcard = true
		}
	}

	requiredSet := make(map[Scope]struct{}, len(required))
	for _, s := range required {
		if s == ScopeWildcard {
			continue
		}

		requiredSet[s] = struct{}{}
	}

	var missing []Scope

	if !hasWildcard {
		for scope := range requiredSet {
			if _, ok := actualSet[scope]; !ok {
				missing = append(missing, scope)
			}
		}
	}

	var excess []Scope

	for scope := range actualSet {
		if scope == ScopeWildcard {
			continue
		}

		if _, ok := requiredSet[scope]; !ok {
			excess = append(excess, scope)
		}
	}

	slices.Sort(missing)
	slices.Sort(excess)

	return ScopeComparison{Missing: missing, Excess: excess}
}
