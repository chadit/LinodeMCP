package tools_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The builder state's own operations, read at the surface the generated arm
// calls rather than through a handler.
//
// The handler cases beside these read the sentence a caller sees, which the
// tool declares. What they cannot see is the condition value the operation
// reports, and that value is the whole of what the emitted ladder matches on: a
// method reporting the wrong one answers the wrong sentence with nothing
// failing. These cases read it directly.

// TestDraftCreateReportsTheConditionsItDeclares holds the two conditions
// LOCAL_CALL_DRAFT_CREATE declares to the values its arm compares against.
func TestDraftCreateReportsTheConditionsItDeclares(t *testing.T) {
	t.Parallel()

	state := cloneState(builder.NewRegistry())

	if _, err := state.DraftCreate(draftFixtureName, "no-such-profile"); !errors.Is(
		err, genlocal.ErrLocalNotFound,
	) {
		t.Errorf("a seed nothing resolves reported %v, want the not-found condition", err)
	}

	if _, err := state.DraftCreate(draftFixtureName, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := state.DraftCreate(draftFixtureName, ""); !errors.Is(
		err, genlocal.ErrLocalAlreadyExists,
	) {
		t.Errorf("a name already filed reported %v, want the exists condition", err)
	}
}

// TestDraftCreateSeedsFromTheConfigurationTheStateCarries is the path the
// shipped answer capture never reaches: a seed that resolves.
//
// It is what says the configuration reaches the operation at all. The refusal
// beside it answers the same way for a name nothing carries and for no
// configuration at all, so only a hit tells the two apart.
func TestDraftCreateSeedsFromTheConfigurationTheStateCarries(t *testing.T) {
	t.Parallel()

	answer, err := cloneState(builder.NewRegistry()).DraftCreate(draftFixtureName, cloneSourceName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	source := fixtureSourceProfile()
	if answer.Description != source.Description {
		t.Errorf("description = %q, want the source profile's %q",
			answer.Description, source.Description)
	}

	if len(answer.AllowedTools) != len(source.AllowedTools) {
		t.Errorf("allowed tools = %v, want the source profile's %v",
			answer.AllowedTools, source.AllowedTools)
	}
}

// TestDraftSetReportsTheConditionItDeclares covers the one condition
// LOCAL_CALL_DRAFT_SET declares, on each setting that can report it.
func TestDraftSetReportsTheConditionItDeclares(t *testing.T) {
	t.Parallel()

	state := draftState(builder.NewRegistry())
	environments := []string{envProd}
	scopes := []string{scopeLinodesReadWrite}
	yolo := true

	_, byEnvironments := state.DraftSet(missingDraftName, &environments, nil, nil)
	wantsTheMissingCondition(t, "environments", byEnvironments)

	_, byScopes := state.DraftSet(missingDraftName, nil, &scopes, nil)
	wantsTheMissingCondition(t, profileTokenScopesParam, byScopes)

	_, byYolo := state.DraftSet(missingDraftName, nil, nil, &yolo)
	wantsTheMissingCondition(t, "yolo", byYolo)
}

// missingDraftName is a name no registry in these cases holds, which is what
// reaches the one condition the draft operations declare.
const missingDraftName = "no-such-draft"

// wantsTheMissingCondition asserts one call reported the draft-missing
// condition, naming the setting that made the call.
func wantsTheMissingCondition(t *testing.T, setting string, err error) {
	t.Helper()

	if !errors.Is(err, genlocal.ErrLocalDraftMissing) {
		t.Errorf("%s against a missing draft reported %v, want the missing condition",
			setting, err)
	}
}

// TestDraftSetLeavesASettingTheCallDidNotSendAlone is what the pointers buy: an
// absent setting is not an instruction, so it reaches neither the registry nor
// the answer.
func TestDraftSetLeavesASettingTheCallDidNotSendAlone(t *testing.T) {
	t.Parallel()

	registry := builder.NewRegistry()
	state := draftState(registry)

	if _, err := state.DraftCreate(draftFixtureName, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	yolo := true

	answer, err := state.DraftSet(draftFixtureName, nil, nil, &yolo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(answer.Changes) != 1 {
		t.Errorf("changes = %v, want the one setting the call sent", answer.Changes)
	}

	if _, moved := answer.Changes["allow_yolo"]; !moved {
		t.Errorf("changes = %v, want the sent setting named in it", answer.Changes)
	}
}

// TestTheTwoSidesOfDraftToolsAnswerDifferently is the direction proof. Each
// side has its own arm now, so nothing at run time carries a direction: what
// separates them is which method the emitted arm calls.
//
// The two differ in more than sign. Adding expands its patterns against the
// live catalog, so a wildcard picks up a tool the draft never named, while
// removing matches the draft's own list. A side serving the other one answers
// the wrong member with the wrong contents.
func TestTheTwoSidesOfDraftToolsAnswerDifferently(t *testing.T) {
	t.Parallel()

	state := mutateState(builder.NewRegistry())

	if _, err := state.DraftCreate(draftFixtureName, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	added, err := state.DraftToolsAdd(draftFixtureName, []string{"linode_*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(added.Added) == 0 {
		t.Fatal("the adding side matched nothing, so the catalog never reached it")
	}

	removed, err := state.DraftToolsRemove(draftFixtureName, []string{added.Added[0]})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed.Removed) != 1 || removed.Removed[0] != added.Added[0] {
		t.Errorf("removed = %v, want just %q", removed.Removed, added.Added[0])
	}

	// The catalog is what the two sides disagree about: a pattern the draft
	// does not carry matches on the adding side and nothing on the removing
	// one, so a side serving the other cannot answer this pair.
	empty, err := state.DraftToolsRemove(draftFixtureName, []string{added.Added[0]})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(empty.Removed) != 0 {
		t.Errorf("removing what is already gone answered %v, want nothing", empty.Removed)
	}
}

// TestBothSidesOfDraftToolsReportTheConditionTheyDeclare holds each side to the
// one condition LOCAL_CALL_DRAFT_TOOLS declares.
func TestBothSidesOfDraftToolsReportTheConditionTheyDeclare(t *testing.T) {
	t.Parallel()

	state := mutateState(builder.NewRegistry())

	if _, err := state.DraftToolsAdd(missingDraftName, nil); !errors.Is(
		err, genlocal.ErrLocalDraftMissing,
	) {
		t.Errorf("the adding side reported %v, want the missing condition", err)
	}

	if _, err := state.DraftToolsRemove(missingDraftName, nil); !errors.Is(
		err, genlocal.ErrLocalDraftMissing,
	) {
		t.Errorf("the removing side reported %v, want the missing condition", err)
	}
}

// TestDraftSaveReportsTheConditionsItDeclares holds the four conditions
// LOCAL_CALL_DRAFT_SAVE declares to the values its arm compares against.
//
// The two store conditions carry a cause as well as a condition, which is the
// half a handler case cannot see: the sentence it words names the file, so a
// condition reported bare would read "failed to load config from read failed".
func TestDraftSaveReportsTheConditionsItDeclares(t *testing.T) {
	registry := builder.NewRegistry()

	if _, err := registry.Create(saveDraftName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := draftState(registry)

	t.Setenv(configPathEnv, writableSaveConfig(t))

	if _, err := state.DraftSave(profiles.BuiltinComputeAdmin); !errors.Is(
		err, genlocal.ErrLocalBuiltinProfile,
	) {
		t.Errorf("a built-in name reported %v, want the built-in condition", err)
	}

	if _, err := state.DraftSave(draftNonexistent); !errors.Is(
		err, genlocal.ErrLocalDraftMissing,
	) {
		t.Errorf("a name no draft carries reported %v, want the missing condition", err)
	}

	missing := filepath.Join(t.TempDir(), "absent.yml")
	t.Setenv(configPathEnv, missing)

	if _, err := state.DraftSave(saveDraftName); !errors.Is(
		err, genlocal.ErrLocalReadFailed,
	) {
		t.Errorf("an unreadable config reported %v, want the read condition", err)
	}

	t.Setenv(configPathEnv, readOnlyConfigDir(t))

	if _, err := state.DraftSave(saveDraftName); !errors.Is(
		err, genlocal.ErrLocalWriteFailed,
	) {
		t.Errorf("an unwritable config reported %v, want the write condition", err)
	}
}

// TestDraftSaveCarriesTheFailedPathToTheArm is the other half of the two store
// conditions: which one was met, and the cause beside it.
//
// Read through the arm rather than off the error, because the cause is what the
// arm hands the handler and the sentence the handler words names the file. A
// condition reported with no cause would read "failed to load config from read
// failed" and nothing at the method surface would say so.
func TestDraftSaveCarriesTheFailedPathToTheArm(t *testing.T) {
	registry := builder.NewRegistry()

	if _, err := registry.Create(saveDraftName, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	state := draftState(registry)

	missing := filepath.Join(t.TempDir(), "absent.yml")
	t.Setenv(configPathEnv, missing)

	read := gentools.RunDraftSave(state, saveDraftName)
	if read.Refusal != tools.LocalRefusalReadFailed {
		t.Errorf("refusal = %v, want the read one", read.Refusal)
	}

	if !strings.HasPrefix(read.Cause, missing+": ") {
		t.Errorf("read cause = %q, want it opening on %q", read.Cause, missing)
	}

	sealed := readOnlyConfigDir(t)
	t.Setenv(configPathEnv, sealed)

	write := gentools.RunDraftSave(state, saveDraftName)
	if write.Refusal != tools.LocalRefusalWriteFailed {
		t.Errorf("refusal = %v, want the write one", write.Refusal)
	}

	if !strings.HasPrefix(write.Cause, sealed+": ") {
		t.Errorf("write cause = %q, want it opening on %q", write.Cause, sealed)
	}
}

// TestDraftSaveAnswersWhatTheWriteChanged covers the answer both saves fill:
// the first files a profile that was not there, the second replaces it and
// reports what moved.
func TestDraftSaveAnswersWhatTheWriteChanged(t *testing.T) {
	registry := builder.NewRegistry()

	draft, err := registry.Create(saveDraftName, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	draft.AllowedTools = []string{toolHello}

	state := draftState(registry)

	t.Setenv(configPathEnv, writableSaveConfig(t))

	first, err := state.DraftSave(saveDraftName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !first.IsNew {
		t.Error("the first save answered IsNew false, want true")
	}

	if !slices.Equal(first.AddedTools, []string{toolHello}) {
		t.Errorf("added = %v, want %v", first.AddedTools, []string{toolHello})
	}

	draft.AllowedTools = []string{toolInstanceBoot}
	draft.AllowYolo = true

	second, err := state.DraftSave(saveDraftName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if second.IsNew {
		t.Error("the second save answered IsNew true, want false")
	}

	if !slices.Equal(second.AddedTools, []string{toolInstanceBoot}) {
		t.Errorf("added = %v, want %v", second.AddedTools, []string{toolInstanceBoot})
	}

	if !slices.Equal(second.RemovedTools, []string{toolHello}) {
		t.Errorf("removed = %v, want %v", second.RemovedTools, []string{toolHello})
	}

	if _, moved := second.ChangedFields[keyAllowYolo]; !moved {
		t.Errorf("changed = %v, want the flag among them", second.ChangedFields)
	}
}
