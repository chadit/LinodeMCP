package builder_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/profiles/builder"
)

// fixtureProfile returns a non-empty Profile used as the clone source in
// the clone-from tests. Tools, environments, scopes, and yolo are all
// populated so we can verify each field copies independently and that
// later mutation of the draft doesn't leak back into the source.
func fixtureProfile(t *testing.T) *profiles.Profile {
	t.Helper()

	return &profiles.Profile{
		Name:                "source",
		Description:         "Source profile for clone tests",
		AllowedTools:        []string{"linode_instance_list", "linode_account"},
		AllowedEnvironments: []string{"prod"},
		RequiredTokenScopes: []string{"linodes:read_only", "account:read_only"},
		AllowYolo:           true,
	}
}

// TestNewRegistryStartsEmpty locks in the construction contract:
// freshly-built registries have zero drafts. Phase 8.4 mutation handlers
// rely on List() being callable without seeding.
func TestNewRegistryStartsEmpty(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	requireNotNil(t, reg, "NewRegistry must return a usable registry")
	assertEmpty(t, reg.List(), "new registry must hold zero drafts")
}

// TestCreateMinimalDraftFromScratch covers the no-clone-from path: a
// brand-new draft has the given name and empty everything else. Phase 8.3
// `_new` without `clone_from` flows through this code path.
func TestCreateMinimalDraftFromScratch(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create("dns-readall", nil)

	requireNoError(t, err)
	requireNotNil(t, draft)
	assertEqual(t, "dns-readall", draft.Name)
	assertEmpty(t, draft.Description)
	assertEmpty(t, draft.AllowedTools)
	assertEmpty(t, draft.AllowedEnvironments)
	assertEmpty(t, draft.RequiredTokenScopes)
	assertFalse(t, draft.AllowYolo)
}

// TestCreateClonesAllFieldsFromProfile verifies copy fidelity. Every
// field on the source Profile lands on the Draft. Phase 8.3 `_new` with
// `clone_from` flows through this code path and the model expects the
// new draft to mirror the source.
func TestCreateClonesAllFieldsFromProfile(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	src := fixtureProfile(t)

	draft, err := reg.Create("my-dns", src)

	requireNoError(t, err)
	requireNotNil(t, draft)
	assertEqual(t, "my-dns", draft.Name)
	assertEqual(t, src.Description, draft.Description)
	assertEqual(t, src.AllowedTools, draft.AllowedTools)
	assertEqual(t, src.AllowedEnvironments, draft.AllowedEnvironments)
	assertEqual(t, src.RequiredTokenScopes, draft.RequiredTokenScopes)
	assertEqual(t, src.AllowYolo, draft.AllowYolo)
}

// TestCreateClonedDraftIsolatesFromSource verifies that mutating the
// draft's slices after a clone does NOT leak back into the source
// profile. Without slices.Clone the underlying arrays would alias and a
// `_add_tools` against the draft would silently modify the built-in
// catalog. That's the exact bug this test guards.
func TestCreateClonedDraftIsolatesFromSource(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	src := fixtureProfile(t)
	originalTools := append([]string(nil), src.AllowedTools...)

	draft, err := reg.Create("my-dns", src)
	requireNoError(t, err)

	draft.AllowedTools = append(draft.AllowedTools, "linode_domain_list")

	assertEqual(t, originalTools, src.AllowedTools,
		"draft mutation must not propagate to source profile")
	assertLen(t, draft.AllowedTools, len(originalTools)+1,
		"draft must hold the additional tool")
}

// TestCreateRefusesEmptyName covers the validation guard. An empty draft
// name would yield a config map entry with a blank key on save; refuse
// at create time so the failure surfaces near the user's mistake.
func TestCreateRefusesEmptyName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create("", nil)

	requireErrorIs(t, err, builder.ErrDraftNameEmpty)
	assertNil(t, draft)
}

// TestCreateRefusesDuplicateName locks in the no-silent-overwrite rule:
// a second Create with the same name returns ErrDraftExists. The user
// must Discard first or pick a different name.
func TestCreateRefusesDuplicateName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create("dns-readall", nil)
	requireNoError(t, err)

	dup, err := reg.Create("dns-readall", nil)

	requireErrorIs(t, err, builder.ErrDraftExists)
	assertNil(t, dup)
}

// TestGetReturnsLiveDraft verifies that Get returns the same pointer
// Create produced, so Phase 8.4 mutators can locate and edit the draft.
func TestGetReturnsLiveDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	original, err := reg.Create("dns-readall", nil)
	requireNoError(t, err)

	got, ok := reg.Get("dns-readall")

	requireTrue(t, ok)
	assertSame(t, original, got, "Get must return the registry's own draft pointer")
}

// TestGetMissingReturnsFalse covers the not-found path. Tool handlers
// rely on the boolean to produce a friendly "no such draft" error.
func TestGetMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	got, ok := reg.Get("nonexistent")

	assertFalse(t, ok)
	assertNil(t, got)
}

// TestDiscardRemovesDraft covers the happy path. After discard the
// draft is gone from List and Get returns false.
func TestDiscardRemovesDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create("dns-readall", nil)
	requireNoError(t, err)

	removed := reg.Discard("dns-readall")

	assertTrue(t, removed, "Discard must report removal of an existing draft")
	assertEmpty(t, reg.List(), "discarded draft must not appear in List")
	_, ok := reg.Get("dns-readall")
	assertFalse(t, ok, "discarded draft must not be retrievable via Get")
}

// TestDiscardMissingIsIdempotent locks in the idempotent contract: a
// discard against a name that was never created returns false rather
// than erroring. Tool handlers can call Discard on tear-down paths
// without first checking existence.
func TestDiscardMissingIsIdempotent(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	removed := reg.Discard("nonexistent")

	assertFalse(t, removed)
}

// TestListReturnsSortedNames locks in the sort contract. Stable output
// matters for Phase 8.3 `_show` output and for the `_save` diff
// presentation; both compare draft names against existing profile names.
func TestListReturnsSortedNames(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()
	_, err := reg.Create("zebra", nil)
	requireNoError(t, err)
	_, err = reg.Create("alpha", nil)
	requireNoError(t, err)
	_, err = reg.Create("middle", nil)
	requireNoError(t, err)

	names := reg.List()

	assertEqual(t, []string{"alpha", "middle", "zebra"}, names)
}

// TestListEmptyRegistryReturnsEmptySlice locks the non-nil contract:
// List on an empty registry returns a usable empty slice, not nil. JSON
// marshaling of the `_show` response surfaces as `[]` not `null`, which
// the spec contract expects.
func TestListEmptyRegistryReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	names := reg.List()

	requireNotNil(t, names, "List must return a non-nil slice")
	assertEmpty(t, names)
}
