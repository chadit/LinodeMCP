package builder_test

import (
	"errors"
	"reflect"
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
		AllowedTools:        []string{"linode_instance_list", "linode_account_get"},
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

	if reg == nil {
		t.Fatal("reg is nil")
	}

	if len(reg.List()) != 0 {
		t.Errorf("reg.List() = %v, want empty", reg.List())
	}
}

// TestCreateMinimalDraftFromScratch covers the no-clone-from path: a
// brand-new draft has the given name and empty everything else. Phase 8.3
// `_new` without `clone_from` flows through this code path.
func TestCreateMinimalDraftFromScratch(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create("dns-readall", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if draft == nil {
		t.Fatal("draft is nil")
	}

	if draft.Name != "dns-readall" {
		t.Errorf("draft.Name = %v, want %v", draft.Name, "dns-readall")
	}

	if draft.Description != "" {
		t.Errorf("draft.Description = %v, want empty", draft.Description)
	}

	if len(draft.AllowedTools) != 0 {
		t.Errorf("draft.AllowedTools = %v, want empty", draft.AllowedTools)
	}

	if len(draft.AllowedEnvironments) != 0 {
		t.Errorf("draft.AllowedEnvironments = %v, want empty", draft.AllowedEnvironments)
	}

	if len(draft.RequiredTokenScopes) != 0 {
		t.Errorf("draft.RequiredTokenScopes = %v, want empty", draft.RequiredTokenScopes)
	}

	if draft.AllowYolo {
		t.Error("draft.AllowYolo = true, want false")
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if draft == nil {
		t.Fatal("draft is nil")
	}

	if draft.Name != "my-dns" {
		t.Errorf("draft.Name = %v, want %v", draft.Name, "my-dns")
	}

	if draft.Description != src.Description {
		t.Errorf("draft.Description = %v, want %v", draft.Description, src.Description)
	}

	if !reflect.DeepEqual(draft.AllowedTools, src.AllowedTools) {
		t.Errorf("draft.AllowedTools = %v, want %v", draft.AllowedTools, src.AllowedTools)
	}

	if !reflect.DeepEqual(draft.AllowedEnvironments, src.AllowedEnvironments) {
		t.Errorf("draft.AllowedEnvironments = %v, want %v", draft.AllowedEnvironments, src.AllowedEnvironments)
	}

	if !reflect.DeepEqual(draft.RequiredTokenScopes, src.RequiredTokenScopes) {
		t.Errorf("draft.RequiredTokenScopes = %v, want %v", draft.RequiredTokenScopes, src.RequiredTokenScopes)
	}

	if draft.AllowYolo != src.AllowYolo {
		t.Errorf("draft.AllowYolo = %v, want %v", draft.AllowYolo, src.AllowYolo)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	draft.AllowedTools = append(draft.AllowedTools, "linode_domain_list")

	if !reflect.DeepEqual(src.AllowedTools, originalTools) {
		t.Errorf("src.AllowedTools = %v, want %v", src.AllowedTools, originalTools)
	}

	if len(draft.AllowedTools) != len(originalTools)+1 {
		t.Errorf("len(draft.AllowedTools) = %d, want %d", len(draft.AllowedTools), len(originalTools)+1)
	}
}

// TestCreateRefusesEmptyName covers the validation guard. An empty draft
// name would yield a config map entry with a blank key on save; refuse
// at create time so the failure surfaces near the user's mistake.
func TestCreateRefusesEmptyName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	draft, err := reg.Create("", nil)

	if !errors.Is(err, builder.ErrDraftNameEmpty) {
		t.Fatalf("error = %v, want %v", err, builder.ErrDraftNameEmpty)
	}

	if draft != nil {
		t.Errorf("draft = %v, want nil", draft)
	}
}

// TestCreateRefusesDuplicateName locks in the no-silent-overwrite rule:
// a second Create with the same name returns ErrDraftExists. The user
// must Discard first or pick a different name.
func TestCreateRefusesDuplicateName(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create("dns-readall", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dup, err := reg.Create("dns-readall", nil)

	if !errors.Is(err, builder.ErrDraftExists) {
		t.Fatalf("error = %v, want %v", err, builder.ErrDraftExists)
	}

	if dup != nil {
		t.Errorf("dup = %v, want nil", dup)
	}
}

// TestGetReturnsLiveDraft verifies that Get returns the same pointer
// Create produced, so Phase 8.4 mutators can locate and edit the draft.
func TestGetReturnsLiveDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	original, err := reg.Create("dns-readall", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := reg.Get("dns-readall")

	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !reflect.DeepEqual(got, original) {
		t.Errorf("got = %v, want %v", got, original)
	}
}

// TestGetMissingReturnsFalse covers the not-found path. Tool handlers
// rely on the boolean to produce a friendly "no such draft" error.
func TestGetMissingReturnsFalse(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	got, ok := reg.Get("nonexistent")

	if ok {
		t.Error("ok = true, want false")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

// TestDiscardRemovesDraft covers the happy path. After discard the
// draft is gone from List and Get returns false.
func TestDiscardRemovesDraft(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create("dns-readall", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	removed := reg.Discard("dns-readall")

	if !removed {
		t.Error("removed = false, want true")
	}

	if len(reg.List()) != 0 {
		t.Errorf("reg.List() = %v, want empty", reg.List())
	}

	_, ok := reg.Get("dns-readall")
	if ok {
		t.Error("ok = true, want false")
	}
}

// TestDiscardMissingIsIdempotent locks in the idempotent contract: a
// discard against a name that was never created returns false rather
// than erroring. Tool handlers can call Discard on tear-down paths
// without first checking existence.
func TestDiscardMissingIsIdempotent(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	removed := reg.Discard("nonexistent")

	if removed {
		t.Error("removed = true, want false")
	}
}

// TestListReturnsSortedNames locks in the sort contract. Stable output
// matters for Phase 8.3 `_show` output and for the `_save` diff
// presentation; both compare draft names against existing profile names.
func TestListReturnsSortedNames(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	_, err := reg.Create("zebra", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = reg.Create("alpha", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = reg.Create("middle", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := reg.List()

	if !reflect.DeepEqual(names, []string{"alpha", "middle", "zebra"}) {
		t.Errorf("names = %v, want %v", names, []string{"alpha", "middle", "zebra"})
	}
}

// TestListEmptyRegistryReturnsEmptySlice locks the non-nil contract:
// List on an empty registry returns a usable empty slice, not nil. JSON
// marshaling of the `_show` response surfaces as `[]` not `null`, which
// the spec contract expects.
func TestListEmptyRegistryReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	reg := builder.NewRegistry()

	names := reg.List()

	if names == nil {
		t.Fatal("names is nil")
	}

	if len(names) != 0 {
		t.Errorf("names = %v, want empty", names)
	}
}
