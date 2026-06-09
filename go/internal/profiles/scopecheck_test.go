package profiles_test

import (
	"reflect"
	"slices"
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

	if profiles.ParsePATScopes("") != nil {
		t.Errorf("value = %v, want nil", profiles.ParsePATScopes(""))
	}

	if profiles.ParsePATScopes("   ") != nil {
		t.Errorf("value = %v, want nil", profiles.ParsePATScopes("   "))
	}

	if profiles.ParsePATScopes("\t\n") != nil {
		t.Errorf("value = %v, want nil", profiles.ParsePATScopes("\t\n"))
	}
}

// TestParsePATScopesSplits the canonical PAT format: space-delimited
// scope strings round-trip into a deduplicated sorted Scope slice.
func TestParsePATScopesSplits(t *testing.T) {
	t.Parallel()

	got := profiles.ParsePATScopes("linodes:read_write volumes:read_only domains:read_write")

	{
		gotEls := slices.Clone(got)
		wantEls := slices.Clone([]profiles.Scope{
			profiles.ScopeLinodesReadWrite,
			profiles.ScopeVolumesReadOnly,
			profiles.ScopeDomainsReadWrite,
		})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", got, []profiles.Scope{
				profiles.ScopeLinodesReadWrite,
				profiles.ScopeVolumesReadOnly,
				profiles.ScopeDomainsReadWrite,
			})
		}
	}
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
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want %d", len(got), 2)
	}

	if !slices.Contains(got, profiles.ScopeLinodesReadWrite) {
		t.Errorf("got does not contain %v", profiles.ScopeLinodesReadWrite)
	}

	if !slices.Contains(got, profiles.ScopeVolumesReadOnly) {
		t.Errorf("got does not contain %v", profiles.ScopeVolumesReadOnly)
	}
}

// TestParsePATScopesPreservesWildcard locks in that "*" (the all-access
// scope marker) parses straight through to ScopeWildcard so the
// downstream comparison logic can short-circuit on it.
func TestParsePATScopesPreservesWildcard(t *testing.T) {
	t.Parallel()

	got := profiles.ParsePATScopes("*")
	if !reflect.DeepEqual(got, []profiles.Scope{profiles.ScopeWildcard}) {
		t.Errorf("got = %v, want %v", got, []profiles.Scope{profiles.ScopeWildcard})
	}
}

// TestFlattenGrantsNil verifies the nil-grants safety contract. A token
// inspection that returns nil (network failure, malformed response)
// must yield nil rather than panicking the loader.
func TestFlattenGrantsNil(t *testing.T) {
	t.Parallel()

	if profiles.FlattenGrants(nil) != nil {
		t.Errorf("profiles.FlattenGrants(nil) = %v, want nil", profiles.FlattenGrants(nil))
	}
}

// TestFlattenGrantsEmpty covers the PAT path: PATs return a 200 with
// zero-valued Grants from /profile/grants. The flattener must produce
// an empty Scope set (not an error).
func TestFlattenGrantsEmpty(t *testing.T) {
	t.Parallel()

	if len(profiles.FlattenGrants(&linode.Grants{})) != 0 {
		t.Errorf("value = %v, want empty", profiles.FlattenGrants(&linode.Grants{}))
	}
}

// TestFlattenGrantsGlobalAccess walks the GlobalGrants AccountAccess
// permission. "read_write" implies both :read_only and :read_write on
// account; "read_only" implies only :read_only.
func TestFlattenGrantsGlobalAccess(t *testing.T) {
	t.Parallel()

	readWriteScopes := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{AccountAccess: permReadWrite},
	})
	if !slices.Contains(readWriteScopes, profiles.ScopeAccountReadOnly) {
		t.Errorf("readWriteScopes does not contain %v", profiles.ScopeAccountReadOnly)
	}

	if !slices.Contains(readWriteScopes, profiles.ScopeAccountReadWrite) {
		t.Errorf("readWriteScopes does not contain %v", profiles.ScopeAccountReadWrite)
	}

	readOnlyScopes := profiles.FlattenGrants(&linode.Grants{
		Global: linode.GlobalGrants{AccountAccess: permReadOnly},
	})
	if !slices.Contains(readOnlyScopes, profiles.ScopeAccountReadOnly) {
		t.Errorf("readOnlyScopes does not contain %v", profiles.ScopeAccountReadOnly)
	}

	if slices.Contains(readOnlyScopes, profiles.ScopeAccountReadWrite) {
		t.Errorf("readOnlyScopes should not contain %v", profiles.ScopeAccountReadWrite)
	}
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
		if !slices.Contains(got, want) {
			t.Errorf("got does not contain %v", want)
		}
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

	if !slices.Contains(got, profiles.ScopeLinodesReadWrite) {
		t.Errorf("got does not contain %v", profiles.ScopeLinodesReadWrite)
	}

	if !slices.Contains(got, profiles.ScopeLinodesReadOnly) {
		t.Errorf("got does not contain %v", profiles.ScopeLinodesReadOnly)
	}

	if !slices.Contains(got, profiles.ScopeDomainsReadOnly) {
		t.Errorf("got does not contain %v", profiles.ScopeDomainsReadOnly)
	}

	if slices.Contains(got, profiles.ScopeDomainsReadWrite) {
		t.Errorf("got should not contain %v", profiles.ScopeDomainsReadWrite)
	}

	if slices.Contains(got, profiles.ScopeVolumesReadOnly) {
		t.Errorf("got should not contain %v", profiles.ScopeVolumesReadOnly)
	}
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

	if count != 2 {
		t.Errorf("count = %v, want %v", count, 2)
	}
}

// TestCompareScopesAllPresent covers the happy path: every required
// scope is present, no excess. Both Missing and Excess are empty.
func TestCompareScopesAllPresent(t *testing.T) {
	t.Parallel()

	got := profiles.CompareScopes(
		[]profiles.Scope{profiles.ScopeLinodesReadOnly, profiles.ScopeVolumesReadOnly},
		[]profiles.Scope{profiles.ScopeLinodesReadOnly, profiles.ScopeVolumesReadOnly},
	)

	if got.HasMissing() {
		t.Error("got.HasMissing() = true, want false")
	}

	if got.HasExcess() {
		t.Error("got.HasExcess() = true, want false")
	}
}

// TestCompareScopesMissingReportsGap verifies the missing-set path:
// the token lacks a scope the profile needs. Missing is sorted so the
// error message stays reproducible.
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

	if !got.HasMissing() {
		t.Error("got.HasMissing() = false, want true")
	}

	if !reflect.DeepEqual(got.Missing, []profiles.Scope{
		profiles.ScopeDomainsReadWrite,
		profiles.ScopeVolumesReadOnly,
	}) {
		t.Errorf("got.Missing = %v, want %v", got.Missing, []profiles.Scope{
			profiles.ScopeDomainsReadWrite,
			profiles.ScopeVolumesReadOnly,
		})
	}
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

	if got.HasMissing() {
		t.Error("got.HasMissing() = true, want false")
	}

	if !got.HasExcess() {
		t.Error("got.HasExcess() = false, want true")
	}

	if !reflect.DeepEqual(got.Excess, []profiles.Scope{profiles.ScopeVolumesReadWrite}) {
		t.Errorf("got.Excess = %v, want %v", got.Excess, []profiles.Scope{profiles.ScopeVolumesReadWrite})
	}
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

	if got.HasMissing() {
		t.Error("got.HasMissing() = true, want false")
	}

	if got.HasExcess() {
		t.Error("got.HasExcess() = true, want false")
	}
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

	if got.HasMissing() {
		t.Error("got.HasMissing() = true, want false")
	}
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

	if got.HasMissing() {
		t.Error("got.HasMissing() = true, want false")
	}

	if !got.HasExcess() {
		t.Error("got.HasExcess() = false, want true")
	}
}
