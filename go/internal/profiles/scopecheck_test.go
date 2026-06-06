package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// Linode grant permission strings used across the scopecheck tests.
// Pulled out so goconst doesn't flag the same literal recurring across
// the FlattenGrants test fixtures.
const (
	permReadOnly  linode.GrantPermission = "read_only"
	permReadWrite linode.GrantPermission = "read_write"
)

// TestParsePATScopesEmpty covers the empty-input edge cases. A PAT with
// no scopes (or whitespace-only) returns nil so the loader's PAT-vs-
// OAuth signal stays useful (empty Profile.Scopes is the OAuth path
// marker, but a parsed-empty string also has to behave).
func TestParsePATScopesEmpty(t *testing.T) {
	t.Parallel()

	assertNil(t, profiles.ParsePATScopes(""))
	assertNil(t, profiles.ParsePATScopes("   "))
	assertNil(t, profiles.ParsePATScopes("\t\n"))
}

// TestParsePATScopesSplits the canonical PAT format: space-delimited
// scope strings round-trip into a deduplicated sorted Scope slice.
func TestParsePATScopesSplits(t *testing.T) {
	t.Parallel()

	got := profiles.ParsePATScopes("linodes:read_write volumes:read_only domains:read_write")

	assertElementsMatch(
		t,
		[]profiles.Scope{
			profiles.ScopeLinodesReadWrite,
			profiles.ScopeVolumesReadOnly,
			profiles.ScopeDomainsReadWrite,
		},
		got,
	)
}

// TestParsePATScopesDedupes verifies that a malformed PAT response with
// duplicates collapses to a single Scope entry per value. Pin this so a
// future API drift that ships repeats doesn't blow up downstream
// comparisons. The input string is built via Sprintf so dupword (which
// scans source literals for repeats) doesn't trip on the intentional
// duplicate.
func TestParsePATScopesDedupes(t *testing.T) {
	t.Parallel()

	scope := "linodes:read_write"
	input := scope + " " + scope + " volumes:read_only"

	got := profiles.ParsePATScopes(input)
	assertLen(t, got, 2)
	assertContains(t, got, profiles.ScopeLinodesReadWrite)
	assertContains(t, got, profiles.ScopeVolumesReadOnly)
}

// TestParsePATScopesPreservesWildcard locks in that "*" (the all-access
// scope marker) parses straight through to ScopeWildcard so the
// downstream comparison logic can short-circuit on it.
func TestParsePATScopesPreservesWildcard(t *testing.T) {
	t.Parallel()

	got := profiles.ParsePATScopes("*")
	assertEqual(t, []profiles.Scope{profiles.ScopeWildcard}, got)
}

// TestFlattenGrantsNil verifies the nil-grants safety contract. A token
// inspection that returns nil (network failure, malformed response)
// must yield nil rather than panicking the loader.
func TestFlattenGrantsNil(t *testing.T) {
	t.Parallel()

	assertNil(t, profiles.FlattenGrants(nil))
}

// TestFlattenGrantsEmpty covers the PAT path: PATs return a 200 with
// zero-valued Grants from /profile/grants. The flattener must produce
// an empty Scope set (not an error).
func TestFlattenGrantsEmpty(t *testing.T) {
	t.Parallel()

	assertEmpty(t, profiles.FlattenGrants(&linode.Grants{}))
}

// TestFlattenGrantsGlobalAccess walks the GlobalGrants AccountAccess
// permission. "read_write" implies both :read_only and :read_write on
// account; "read_only" implies only :read_only.
func TestFlattenGrantsGlobalAccess(t *testing.T) {
	t.Parallel()

	rw := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{AccountAccess: permReadWrite},
	})
	assertContains(t, rw, profiles.ScopeAccountReadOnly)
	assertContains(t, rw, profiles.ScopeAccountReadWrite)

	ro := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{AccountAccess: permReadOnly},
	})
	assertContains(t, ro, profiles.ScopeAccountReadOnly)
	assertNotContains(t, ro, profiles.ScopeAccountReadWrite,
		"read_only must not imply write")
}

// TestFlattenGrantsAddFlags covers the per-resource Add* booleans. An
// OAuth token that can "add linodes" gets linodes:read_write; the
// :read_only pair is also implied (write subsumes read).
func TestFlattenGrantsAddFlags(t *testing.T) {
	t.Parallel()

	got := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{
			AddLinodes:       true,
			AddDomains:       true,
			AddFirewalls:     true,
			AddImages:        true,
			AddNodeBalancers: true,
			AddStackScripts:  true,
			AddVolumes:       true,
			AddVPCs:          true,
		},
	})

	for _, want := range []profiles.Scope{
		profiles.ScopeLinodesReadWrite,
		profiles.ScopeDomainsReadWrite,
		profiles.ScopeFirewallReadWrite,
		profiles.ScopeImagesReadWrite,
		profiles.ScopeNodeBalancersReadWrite,
		profiles.ScopeStackScriptsReadWrite,
		profiles.ScopeVolumesReadWrite,
		profiles.ScopeVPCReadWrite,
	} {
		assertContains(t, got, want)
	}
}

// TestFlattenGrantsPerResource covers the per-resource grant lists.
// A grant with permission "read_write" must contribute both :read_only
// and :read_write to the flattened set; "read_only" only contributes
// :read_only.
func TestFlattenGrantsPerResource(t *testing.T) {
	t.Parallel()

	got := profiles.FlattenGrants(&linode.Grants{
		Linode: []linode.Grant{
			{ID: 1, Label: "web-1", Permissions: permReadWrite},
		},
		Domain: []linode.Grant{
			{ID: 1, Label: "example.com", Permissions: permReadOnly},
		},
		Volume: []linode.Grant{
			{ID: 1, Label: "data", Permissions: ""},
		},
	})

	assertContains(t, got, profiles.ScopeLinodesReadWrite)
	assertContains(t, got, profiles.ScopeLinodesReadOnly)
	assertContains(t, got, profiles.ScopeDomainsReadOnly)
	assertNotContains(t, got, profiles.ScopeDomainsReadWrite,
		"read_only domain grant must not imply :read_write")
	assertNotContains(t, got, profiles.ScopeVolumesReadOnly,
		"empty permissions grant must contribute nothing")
}

// TestFlattenGrantsSortedDeduplicated guards against an upstream API
// quirk where the same Linode or Domain appears more than once. The
// output is a Scope set, so duplicates collapse and order is stable.
func TestFlattenGrantsSortedDeduplicated(t *testing.T) {
	t.Parallel()

	got := profiles.FlattenGrants(&linode.Grants{
		Linode: []linode.Grant{
			{ID: 1, Permissions: permReadWrite},
			{ID: 2, Permissions: permReadWrite},
			{ID: 3, Permissions: permReadOnly},
		},
	})

	var count int

	for _, s := range got {
		if s == profiles.ScopeLinodesReadWrite || s == profiles.ScopeLinodesReadOnly {
			count++
		}
	}

	assertEqual(t, 2, count, "should produce exactly two linode scopes (no duplicates)")
}

// TestCompareScopesAllPresent covers the happy path: every required
// scope is present, no excess. Both Missing and Excess are empty.
func TestCompareScopesAllPresent(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{profiles.ScopeLinodesReadOnly, profiles.ScopeVolumesReadOnly},
		[]profiles.Scope{profiles.ScopeLinodesReadOnly, profiles.ScopeVolumesReadOnly},
	)

	assertFalse(t, got.HasMissing())
	assertFalse(t, got.HasExcess())
}

// TestCompareScopesMissingReportsGap verifies the missing-set path:
// the token lacks a scope the profile needs. Missing is sorted so the
// error message stays deterministic.
func TestCompareScopesMissingReportsGap(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{
			profiles.ScopeLinodesReadWrite,
			profiles.ScopeVolumesReadOnly,
			profiles.ScopeDomainsReadWrite,
		},
		[]profiles.Scope{profiles.ScopeLinodesReadWrite},
	)

	assertTrue(t, got.HasMissing())
	assertEqual(
		t,
		[]profiles.Scope{
			profiles.ScopeDomainsReadWrite,
			profiles.ScopeVolumesReadOnly,
		},
		got.Missing,
	)
}

// TestCompareScopesExcessIsLeastPrivilegeSignal covers the excess case:
// the token has scopes the profile doesn't need. By default this is a
// warn (not a fail); strict-mode policy lives in the loader, not the
// primitive.
func TestCompareScopesExcessIsLeastPrivilegeSignal(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{profiles.ScopeLinodesReadOnly},
		[]profiles.Scope{
			profiles.ScopeLinodesReadOnly,
			profiles.ScopeVolumesReadWrite,
		},
	)

	assertFalse(t, got.HasMissing())
	assertTrue(t, got.HasExcess())
	assertEqual(t, []profiles.Scope{profiles.ScopeVolumesReadWrite}, got.Excess)
}

// TestCompareScopesWildcardMatchesEverything verifies that a token
// carrying "*" satisfies any required scope. PATs created with the
// "*" scope at the dashboard land here.
func TestCompareScopesWildcardMatchesEverything(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{
			profiles.ScopeLinodesReadWrite,
			profiles.ScopeVolumesReadWrite,
			profiles.ScopeDomainsReadWrite,
		},
		[]profiles.Scope{profiles.ScopeWildcard},
	)

	assertFalse(t, got.HasMissing(),
		"wildcard token must satisfy every required scope")
	assertFalse(t, got.HasExcess(),
		"wildcard alone is not 'excess' since it is the literal grant")
}

// TestCompareScopesRequiredWildcardIsNoOp confirms a profile with "*"
// in its required list passes regardless of token content. The
// derivation in BuiltinProfiles never produces this shape, but a
// user-defined profile could.
func TestCompareScopesRequiredWildcardIsNoOp(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{profiles.ScopeWildcard, profiles.ScopeLinodesReadOnly},
		[]profiles.Scope{profiles.ScopeLinodesReadOnly},
	)

	assertFalse(t, got.HasMissing(),
		"wildcard in required list must not produce a missing entry")
}

// TestCompareScopesEmptyRequiredAlwaysPasses pins the no-op case: a
// profile that declares no required scopes always passes. The
// best-effort fallback in RequiredScopes lands here for unknown tools.
func TestCompareScopesEmptyRequiredAlwaysPasses(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		nil,
		[]profiles.Scope{profiles.ScopeLinodesReadWrite},
	)

	assertFalse(t, got.HasMissing())
	assertTrue(t, got.HasExcess(),
		"a token with scope vs empty required is still 'excess' (least privilege violated)")
}
