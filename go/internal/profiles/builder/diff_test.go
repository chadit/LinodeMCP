package builder_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// TestDraftAsUserProfileCopiesEverySlice checks the save-bridge conversion.
// The config shape is what gets written to disk, so a shared backing array
// would let a later draft edit rewrite an already-saved profile.
func TestDraftAsUserProfileCopiesEverySlice(t *testing.T) {
	t.Parallel()

	draft := &builder.Draft{
		Name:                draftName,
		Description:         descDNSRead,
		AllowedTools:        []string{toolDomainList},
		AllowedEnvironments: []string{envProd},
		RequiredTokenScopes: []string{scopeLinodesRO},
		AllowYolo:           true,
	}

	got := builder.DraftAsUserProfile(draft)

	if got.Description != draft.Description {
		t.Errorf("Description = %q, want %q", got.Description, draft.Description)
	}

	if !got.AllowYolo {
		t.Error("AllowYolo = false, want the draft's true")
	}

	draft.AllowedTools[0] = toolHello
	draft.AllowedEnvironments[0] = envDev
	draft.RequiredTokenScopes[0] = "linodes:read_write"

	if want := []string{toolDomainList}; !reflect.DeepEqual(got.AllowedTools, want) {
		t.Errorf("AllowedTools = %v, want %v after the draft was edited", got.AllowedTools, want)
	}

	if want := []string{envProd}; !reflect.DeepEqual(got.AllowedEnvironments, want) {
		t.Errorf("AllowedEnvironments = %v, want %v after the draft was edited", got.AllowedEnvironments, want)
	}

	if want := []string{scopeLinodesRO}; !reflect.DeepEqual(got.RequiredTokenScopes, want) {
		t.Errorf("RequiredTokenScopes = %v, want %v after the draft was edited", got.RequiredTokenScopes, want)
	}
}

// TestComputeDiffAgainstAbsentProfile covers the new-profile arm: nothing to
// compare against, so every non-zero field reads as a change from its zero
// value and the whole tool list reads as added.
func TestComputeDiffAgainstAbsentProfile(t *testing.T) {
	t.Parallel()

	draftCfg := config.UserProfileConfig{
		Description:         descDNSRead,
		AllowedTools:        []string{toolInstanceList, toolDomainList},
		AllowedEnvironments: []string{envProd},
		AllowYolo:           true,
	}

	diff := builder.ComputeDiff(draftName, &draftCfg, nil)

	if !diff.IsNew {
		t.Error("IsNew = false, want true when no profile existed")
	}

	if diff.Name != draftName {
		t.Errorf("Name = %q, want %q", diff.Name, draftName)
	}

	if want := []string{toolDomainList, toolInstanceList}; !reflect.DeepEqual(diff.AddedTools, want) {
		t.Errorf("AddedTools = %v, want %v sorted", diff.AddedTools, want)
	}

	if len(diff.RemovedTools) != 0 {
		t.Errorf("RemovedTools = %v, want nothing removed for a new profile", diff.RemovedTools)
	}

	wantChanged := []string{"allowed_environments", "allow_yolo", "description"}
	for _, field := range wantChanged {
		if _, ok := diff.ChangedFields[field]; !ok {
			t.Errorf("ChangedFields is missing %q, want every non-zero field reported", field)
		}
	}

	if _, ok := diff.ChangedFields["required_token_scopes"]; ok {
		t.Error("ChangedFields carries required_token_scopes, want an untouched zero field left out")
	}

	if got := diff.ChangedFields["description"]; got.Old != "" || got.New != draftCfg.Description {
		t.Errorf("description diff = %+v, want old empty and new %q", got, draftCfg.Description)
	}
}

// TestComputeDiffReportsOnlyDifferingFields covers the update arm: an existing
// profile means unchanged fields stay out of the change set entirely, which is
// what keeps the save response small enough to summarize.
func TestComputeDiffReportsOnlyDifferingFields(t *testing.T) {
	t.Parallel()

	existing := config.UserProfileConfig{
		Description:         descDNSRead,
		AllowedTools:        []string{toolDomainList, toolHello},
		AllowedEnvironments: []string{envProd},
		RequiredTokenScopes: []string{scopeLinodesRO},
	}
	draftCfg := config.UserProfileConfig{
		Description:         descDNSRead,
		AllowedTools:        []string{toolDomainList, toolInstanceList},
		AllowedEnvironments: []string{envProd},
		RequiredTokenScopes: []string{scopeLinodesRO},
		AllowYolo:           true,
	}

	diff := builder.ComputeDiff(draftName, &draftCfg, &existing)

	if diff.IsNew {
		t.Error("IsNew = true, want false when the profile already existed")
	}

	if want := []string{toolInstanceList}; !reflect.DeepEqual(diff.AddedTools, want) {
		t.Errorf("AddedTools = %v, want %v", diff.AddedTools, want)
	}

	if want := []string{toolHello}; !reflect.DeepEqual(diff.RemovedTools, want) {
		t.Errorf("RemovedTools = %v, want %v", diff.RemovedTools, want)
	}

	if want := 1; len(diff.ChangedFields) != want {
		t.Errorf("ChangedFields = %v, want only the %d field that actually differs", diff.ChangedFields, want)
	}

	yolo, ok := diff.ChangedFields["allow_yolo"]
	if !ok {
		t.Fatalf("ChangedFields = %v, want an allow_yolo entry", diff.ChangedFields)
	}

	if yolo.Old != false || yolo.New != true {
		t.Errorf("allow_yolo diff = %+v, want old false and new true", yolo)
	}
}

// TestComputeDiffRendersEmptyListsNotNull pins the JSON-facing contract: a
// list field that goes from populated to empty reports "[]" on both sides
// rather than a null the model would have to special-case.
func TestComputeDiffRendersEmptyListsNotNull(t *testing.T) {
	t.Parallel()

	existing := config.UserProfileConfig{AllowedEnvironments: []string{envProd}}
	draftCfg := config.UserProfileConfig{}

	diff := builder.ComputeDiff(draftName, &draftCfg, &existing)

	envs, found := diff.ChangedFields["allowed_environments"]
	if !found {
		t.Fatalf("ChangedFields = %v, want an allowed_environments entry", diff.ChangedFields)
	}

	newList, isSlice := envs.New.([]string)
	if !isSlice {
		t.Fatalf("allowed_environments new value is %T, want []string", envs.New)
	}

	if newList == nil {
		t.Error("allowed_environments new value is nil, want an empty slice so it marshals as []")
	}

	if len(newList) != 0 {
		t.Errorf("allowed_environments new value = %v, want empty", newList)
	}

	if len(diff.AddedTools) != 0 || len(diff.RemovedTools) != 0 {
		t.Errorf("tool deltas = %v/%v, want both empty when neither side listed tools",
			diff.AddedTools, diff.RemovedTools)
	}
}
