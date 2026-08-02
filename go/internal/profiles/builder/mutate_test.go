package builder_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// draftName is the name every mutation test creates its draft under.
const draftName = "draft-under-test"

// seededRegistry returns a registry holding one draft named draftName whose
// AllowedTools are the given names, so a mutation test starts from a known
// list rather than from whatever a previous call left behind.
func seededRegistry(t *testing.T, tools ...string) *builder.Registry {
	t.Helper()

	reg := builder.NewRegistry()

	draft, err := reg.Create(draftName, nil)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	if len(tools) > 0 {
		if _, err := reg.AddTools(draftName, tools, matchCatalog()); err != nil {
			t.Fatalf("seed draft tools: %v", err)
		}
	}

	if draft == nil {
		t.Fatal("Create returned a nil draft")
	}

	return reg
}

// draftTools reads the named draft's AllowedTools back through the registry.
func draftTools(t *testing.T, reg *builder.Registry) []string {
	t.Helper()

	draft, ok := reg.Get(draftName)
	if !ok {
		t.Fatalf("draft %q is missing from the registry", draftName)
	}

	return draft.AllowedTools
}

// TestAddToolsMergesMatchesWithoutDuplicates checks what AddTools reports back
// against what it leaves on the draft. The two differ on purpose: the return
// value is only the newly-added names, while the draft carries the merged set.
func TestAddToolsMergesMatchesWithoutDuplicates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seed      []string
		patterns  []string
		wantAdded []string
		wantTools []string
	}{
		{
			name:      "wildcard seeds an empty draft",
			patterns:  []string{instanceGlob},
			wantAdded: []string{toolInstanceGet, toolInstanceList},
			wantTools: []string{toolInstanceGet, toolInstanceList},
		},
		{
			name:      "already-present name is not reported as added",
			seed:      []string{toolInstanceGet},
			patterns:  []string{instanceGlob},
			wantAdded: []string{toolInstanceList},
			wantTools: []string{toolInstanceGet, toolInstanceList},
		},
		{
			name:      "no patterns leaves the draft alone",
			seed:      []string{toolDomainList},
			patterns:  nil,
			wantAdded: []string{},
			wantTools: []string{toolDomainList},
		},
		{
			name:      "pattern matching nothing adds nothing",
			seed:      []string{toolDomainList},
			patterns:  []string{"linode_nope_*"},
			wantAdded: []string{},
			wantTools: []string{toolDomainList},
		},
		{
			name:      "merged tools come back sorted",
			seed:      []string{toolInstanceList},
			patterns:  []string{toolHello, toolDomainList},
			wantAdded: []string{toolHello, toolDomainList},
			wantTools: []string{toolHello, toolDomainList, toolInstanceList},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := seededRegistry(t, tt.seed...)

			added, err := reg.AddTools(draftName, tt.patterns, matchCatalog())
			if err != nil {
				t.Fatalf("AddTools: %v", err)
			}

			if !reflect.DeepEqual(added, tt.wantAdded) {
				t.Errorf("AddTools reported %v as added, want %v", added, tt.wantAdded)
			}

			if got := draftTools(t, reg); !reflect.DeepEqual(got, tt.wantTools) {
				t.Errorf("draft AllowedTools = %v, want %v", got, tt.wantTools)
			}
		})
	}
}

// TestRemoveToolsExpandsAgainstTheDraftNotTheCatalog pins the documented
// difference between the two tool mutators: RemoveTools globs over what the
// draft currently holds, so a name the catalog offers but the draft never
// took is not reported as removed.
func TestRemoveToolsExpandsAgainstTheDraftNotTheCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		seed        []string
		patterns    []string
		wantRemoved []string
		wantTools   []string
	}{
		{
			name:        "wildcard removes every draft match",
			seed:        []string{toolInstanceGet, toolInstanceList, toolDomainList},
			patterns:    []string{instanceGlob},
			wantRemoved: []string{toolInstanceGet, toolInstanceList},
			wantTools:   []string{toolDomainList},
		},
		{
			name:        "catalog name the draft never took is not removed",
			seed:        []string{toolDomainList},
			patterns:    []string{instanceGlob},
			wantRemoved: []string{},
			wantTools:   []string{toolDomainList},
		},
		{
			name:        "literal removes exactly one name",
			seed:        []string{toolHello, toolDomainList},
			patterns:    []string{toolHello},
			wantRemoved: []string{toolHello},
			wantTools:   []string{toolDomainList},
		},
		{
			name:        "bare wildcard empties the draft",
			seed:        []string{toolHello, toolDomainList},
			patterns:    []string{"*"},
			wantRemoved: []string{toolHello, toolDomainList},
			wantTools:   []string{},
		},
		{
			name:        "no patterns removes nothing",
			seed:        []string{toolHello},
			patterns:    nil,
			wantRemoved: []string{},
			wantTools:   []string{toolHello},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := seededRegistry(t, tt.seed...)

			removed, err := reg.RemoveTools(draftName, tt.patterns)
			if err != nil {
				t.Fatalf("RemoveTools: %v", err)
			}

			if !reflect.DeepEqual(removed, tt.wantRemoved) {
				t.Errorf("RemoveTools reported %v as removed, want %v", removed, tt.wantRemoved)
			}

			if got := draftTools(t, reg); !reflect.DeepEqual(got, tt.wantTools) {
				t.Errorf("draft AllowedTools = %v, want %v", got, tt.wantTools)
			}
		})
	}
}

// TestSettersReplaceDraftFields walks the three scalar/list setters. Each one
// replaces rather than merges, and each copies its input so a later edit by the
// caller cannot reach into the stored draft.
func TestSettersReplaceDraftFields(t *testing.T) {
	t.Parallel()

	t.Run("allowed environments replace, not merge", func(t *testing.T) {
		t.Parallel()

		reg := seededRegistry(t)

		if err := reg.SetAllowedEnvironments(draftName, []string{envProd, "staging"}); err != nil {
			t.Fatalf("SetAllowedEnvironments: %v", err)
		}

		if err := reg.SetAllowedEnvironments(draftName, []string{envDev}); err != nil {
			t.Fatalf("SetAllowedEnvironments second call: %v", err)
		}

		draft, ok := reg.Get(draftName)
		if !ok {
			t.Fatal("draft is missing from the registry")
		}

		if want := []string{envDev}; !reflect.DeepEqual(draft.AllowedEnvironments, want) {
			t.Errorf("draft.AllowedEnvironments = %v, want %v", draft.AllowedEnvironments, want)
		}
	})

	t.Run("caller's slice is copied", func(t *testing.T) {
		t.Parallel()

		reg := seededRegistry(t)
		scopes := []string{scopeLinodesRO}

		if err := reg.SetRequiredTokenScopes(draftName, scopes); err != nil {
			t.Fatalf("SetRequiredTokenScopes: %v", err)
		}

		scopes[0] = "linodes:read_write"

		draft, ok := reg.Get(draftName)
		if !ok {
			t.Fatal("draft is missing from the registry")
		}

		if want := []string{scopeLinodesRO}; !reflect.DeepEqual(draft.RequiredTokenScopes, want) {
			t.Errorf("draft.RequiredTokenScopes = %v, want %v after the caller edited its own slice",
				draft.RequiredTokenScopes, want)
		}
	})

	t.Run("yolo flag toggles both ways", func(t *testing.T) {
		t.Parallel()

		reg := seededRegistry(t)

		if err := reg.SetAllowYolo(draftName, true); err != nil {
			t.Fatalf("SetAllowYolo true: %v", err)
		}

		draft, ok := reg.Get(draftName)
		if !ok {
			t.Fatal("draft is missing from the registry")
		}

		if !draft.AllowYolo {
			t.Error("draft.AllowYolo = false, want true after SetAllowYolo(true)")
		}

		if err := reg.SetAllowYolo(draftName, false); err != nil {
			t.Fatalf("SetAllowYolo false: %v", err)
		}

		if draft.AllowYolo {
			t.Error("draft.AllowYolo = true, want false after SetAllowYolo(false)")
		}
	})
}

// wrapDraftErr keeps the not-found table's two-result mutators returning an
// error the table can compare, without handing back the registry's error
// untouched.
func wrapDraftErr(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("registry mutation: %w", err)
}

// TestMutatorsRejectUnknownDraft covers the shared not-found arm. Every mutator
// resolves the draft by name first, so an unknown name must come back as
// ErrDraftNotFound rather than as a silent no-op.
func TestMutatorsRejectUnknownDraft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		call func(reg *builder.Registry) error
		name string
	}{
		{
			name: "AddTools",
			call: func(reg *builder.Registry) error {
				_, err := reg.AddTools("absent", []string{"*"}, matchCatalog())

				return wrapDraftErr(err)
			},
		},
		{
			name: "RemoveTools",
			call: func(reg *builder.Registry) error {
				_, err := reg.RemoveTools("absent", []string{"*"})

				return wrapDraftErr(err)
			},
		},
		{
			name: "SetAllowedEnvironments",
			call: func(reg *builder.Registry) error {
				return reg.SetAllowedEnvironments("absent", []string{envProd})
			},
		},
		{
			name: "SetRequiredTokenScopes",
			call: func(reg *builder.Registry) error {
				return reg.SetRequiredTokenScopes("absent", []string{scopeLinodesRO})
			},
		},
		{
			name: "SetAllowYolo",
			call: func(reg *builder.Registry) error {
				return reg.SetAllowYolo("absent", true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := seededRegistry(t)

			if err := tt.call(reg); !errors.Is(err, builder.ErrDraftNotFound) {
				t.Errorf("%s on an absent draft returned %v, want %v", tt.name, err, builder.ErrDraftNotFound)
			}
		})
	}
}
