package main_test

import (
	"errors"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The refusals the generated arm raises.
//
// An arm is written from the operation's own declaration, so what it can refuse
// is a declaration it cannot write four steps for: a side shaping no answer, a
// condition a reader answers rather than the operation, two directions with no
// state to serve them, and a verdict vocabulary it cannot word. The readings
// and the input types are answered a level up, over their own tables rather
// than per operation, because a kind no language types is a gap whether or not
// an operation declares it. Every case states a synthesized table, because the
// shipped declarations ride on the contract's own LocalCall members and a case
// can perturb none of them.

// shippedVerdicts is a whole, valid wording of the vocabulary, spelled here
// rather than read off the contract so a case perturbing one row is measured
// against a set that would otherwise pass.
//
// The words are stand-ins, not the shipped sentences: what these cases measure
// is the shape of a wording, and pinning the real prose is the shared behavior
// fixture's job.
func shippedVerdicts() []toolgen.ProbeLocalVerdict {
	rows := make([]toolgen.ProbeLocalVerdict, 0, len(toolgen.ProbeVerdictVocabulary()))

	for _, verdict := range toolgen.ProbeVerdictVocabulary() {
		row := toolgen.ProbeLocalVerdict{Verdict: verdict}
		if verdict != probeVerdictPermitted {
			row.Bucket = probeVerdictBucket
			row.Reason = "stopped"
			row.Remedy = "unstop it"
		}

		rows = append(rows, row)
	}

	return rows
}

// probeVerdictPermitted is the one member that stops nothing, so it is the one
// a valid wording leaves wordless, and probeVerdictBucket is the stand-in key
// every other member counts in.
const (
	probeVerdictPermitted = "LOCAL_VERDICT_PERMITTED"
	probeVerdictBucket    = "bucket"
)

// verdictsWithout is that wording with one member's named word emptied, which
// is the shape a row half-stated takes.
func verdictsWithout(verdict, word string) []toolgen.ProbeLocalVerdict {
	rows := shippedVerdicts()
	for index := range rows {
		if rows[index].Verdict != verdict {
			continue
		}

		switch word {
		case "bucket":
			rows[index].Bucket = ""
		case "reason":
			rows[index].Reason = ""
		default:
			rows[index].Remedy = ""
		}
	}

	return rows
}

// verdictsWording is that wording with one member given words it should not
// carry, which is what the permitting verdict is refused for.
func verdictsWording(verdict string) []toolgen.ProbeLocalVerdict {
	rows := shippedVerdicts()
	for index := range rows {
		if rows[index].Verdict == verdict {
			rows[index].Bucket = probeVerdictBucket
		}
	}

	return rows
}

// verdictsNaming is that wording with one member's remedy reading a named
// placeholder, which is how a case states a value no entry carries.
func verdictsNaming(verdict, placeholder string) []toolgen.ProbeLocalVerdict {
	rows := shippedVerdicts()
	for index := range rows {
		if rows[index].Verdict == verdict {
			rows[index].Remedy = "unstop " + placeholder
		}
	}

	return rows
}

// probeGeneratedCall is the operation the perturbing cases rewrite. Every
// declared operation is generated, so a case's own arm reaches the checks.
const probeGeneratedCall = memberDraftRead

// probeDraftShape is the message the perturbing cases answer with, which is the
// one LOCAL_CALL_DRAFT_READ really declares.
func probeDraftShape() map[string]proto.Message {
	return map[string]proto.Message{"": &linodev1.ProfileDraftResponse{}}
}

// TestRefusesGeneratingAnOperationTheArmCannotWrite covers every declaration
// the four emitted steps have no form for.
func TestRefusesGeneratingAnOperationTheArmCannotWrite(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		refusal string
		arm     toolgen.ProbeLocalArm
	}{
		{
			name:    "an operation running two directions with no state to serve them",
			refusal: "errLocalArmSidedStateless",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				TwoSided: true,
				Shapes: map[string]proto.Message{
					probeDirectionAdd:    &linodev1.ProfileDraftAddToolsResponse{},
					probeDirectionRemove: &linodev1.ProfileDraftRemoveToolsResponse{},
				},
			},
		},
		{
			name:    "an operation shaping no answer",
			refusal: "errLocalArmUnshaped",
			arm:     toolgen.ProbeLocalArm{Call: probeGeneratedCall},
		},
		{
			name:    "an operation reporting a condition its reader answers",
			refusal: "errLocalArmReaderGuard",
			arm: toolgen.ProbeLocalArm{
				Call:    probeGeneratedCall,
				Shapes:  probeDraftShape(),
				Reports: []string{"argument_unusable"},
			},
		},
		{
			name:    "an operation wording a verdict row that names no verdict",
			refusal: "errLocalVerdictUnnamed",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: []toolgen.ProbeLocalVerdict{{Verdict: "LOCAL_VERDICT_UNSPECIFIED"}},
			},
		},
		{
			name:    "an operation wording one verdict twice",
			refusal: "errLocalVerdictRepeated",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: append(shippedVerdicts(), shippedVerdicts()[0]),
			},
		},
		{
			name:    "an operation wording part of the verdict vocabulary",
			refusal: "errLocalVerdictIncomplete",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: shippedVerdicts()[:len(shippedVerdicts())-1],
			},
		},
		{
			name:    "an operation stopping an entry with no remedy to clear it",
			refusal: "errLocalVerdictUnworded",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: verdictsWithout("LOCAL_VERDICT_UNREGISTERED", "remedy"),
			},
		},
		{
			name:    "an operation stopping an entry with no bucket to count it in",
			refusal: "errLocalVerdictUnworded",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: verdictsWithout("LOCAL_VERDICT_ENVIRONMENT_BLOCK", "bucket"),
			},
		},
		{
			name:    "an operation wording the verdict that permits an entry",
			refusal: "errLocalVerdictWorded",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: verdictsWording("LOCAL_VERDICT_PERMITTED"),
			},
		},
		{
			name:    "a verdict sentence naming a value no entry carries",
			refusal: "errLocalVerdictPlaceholder",
			arm: toolgen.ProbeLocalArm{
				Call:     probeGeneratedCall,
				Shapes:   probeDraftShape(),
				Verdicts: verdictsNaming("LOCAL_VERDICT_PROFILE_BLOCK", "{profile}"),
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			arms := []toolgen.ProbeLocalArm{testCase.arm}

			err := toolgen.ProbeGeneratedArms(arms)

			if want := refusalNamed(t, testCase.refusal); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want %s (%v)", err, testCase.refusal, want)
			}
		})
	}
}

// TestRefusesAnInputARegisteredLanguageTypesNothingFor covers the other gap a
// tenth language opens on the way in: it registers, states a reader for a kind,
// and gives its subsystem type no spelling to carry the value under.
//
// Reached by taking one row out of a shipped type table, since every kind the
// reader set carries has a row in both today. That is what the call list used
// to be missing, and typing it left the refusal with nothing in the contract
// that could reach it.
func TestRefusesAnInputARegisteredLanguageTypesNothingFor(t *testing.T) {
	t.Parallel()

	for _, language := range []string{probeLanguageGo, probeLanguagePython} {
		t.Run(language, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeOperationInputUntyped(language, "call_list")

			if want := refusalNamed(t, "errLocalArmInputUntyped"); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want errLocalArmInputUntyped (%v)", err, want)
			}
		})
	}
}

// TestTheVerdictVocabularyIsTheTwoValuesAnEntryCarries freezes the value set a
// verdict sentence may read. It is the whole of the guard that keeps a tool's
// own identity out of prose the operation answers, so a member added to it is a
// reviewed decision rather than something a batch does in passing.
func TestTheVerdictVocabularyIsTheTwoValuesAnEntryCarries(t *testing.T) {
	t.Parallel()

	if got := toolgen.ProbeVerdictValues(); !slices.Equal(got, []string{probeToolGlobArg, "capability"}) {
		t.Errorf("the value vocabulary is %v, this case froze the tool and its capability", got)
	}

	if got := toolgen.ProbeVerdictVocabulary(); !slices.Equal(got, []string{
		"LOCAL_VERDICT_PERMITTED", "LOCAL_VERDICT_UNREGISTERED",
		"LOCAL_VERDICT_PROFILE_BLOCK", "LOCAL_VERDICT_CAPABILITY_BLOCK",
		"LOCAL_VERDICT_ENVIRONMENT_BLOCK",
	}) {
		t.Errorf("the verdict vocabulary is %v, this case froze five members", got)
	}
}

// TestRefusesAReadingARegisteredLanguageRendersNothingFor covers the gap a
// tenth language opens: it joins the registry and states no rendering for a
// reading, so its arm hands the subsystem less than the declaration says while
// reading as though it had handed over everything.
func TestRefusesAReadingARegisteredLanguageRendersNothingFor(t *testing.T) {
	t.Parallel()

	err := toolgen.ProbeAmbientRenderings([]string{probeLanguageGo, probeLanguageUnruled})

	if want := refusalNamed(t, "errLocalArmAmbientUnrendered"); !errors.Is(err, want) {
		t.Errorf("refusal = %v, want errLocalArmAmbientUnrendered (%v)", err, want)
	}
}

// TestEveryRegisteredLanguageRendersEveryReading is the case that says the one
// above measures something: the two shipped languages carry the whole
// vocabulary, so the refusal fires on the perturbation rather than on the
// contract as it stands.
func TestEveryRegisteredLanguageRendersEveryReading(t *testing.T) {
	t.Parallel()

	if err := toolgen.ProbeAmbientRenderings(
		[]string{probeLanguageGo, probeLanguagePython},
	); err != nil {
		t.Errorf("the shipped languages refused: %v", err)
	}
}

// TestEveryReadingRendersAsTheEngineTakesIt pins each reading's per-language
// spelling to the local the engines really hold it under, and to the expression
// their handlers hand over.
//
// Spelled out rather than read back off the table, and the vocabulary is frozen
// beside it: a reading added to the contract lands here as a failing row rather
// than as a member every language quietly renders in nothing.
func TestEveryReadingRendersAsTheEngineTakesIt(t *testing.T) {
	t.Parallel()

	if got := toolgen.ProbeAmbientVocabulary(); !slices.Equal(got, []string{
		"LOCAL_AMBIENT_CANCELLATION", "LOCAL_AMBIENT_CONFIG", "LOCAL_AMBIENT_CLOCK",
	}) {
		t.Errorf("the reading vocabulary is %v, this case froze cancellation, config and the clock", got)
	}

	cases := []struct {
		language string
		reading  string
		local    string
		handed   string
	}{
		{probeLanguageGo, "cancellation", "ctx", "ctx"},
		{probeLanguageGo, "config", probeConfigLocal, probeConfigLocal},
		// The clock is the reading neither handler holds: each hands over what
		// its own engine reads the reference instant from.
		{probeLanguageGo, "clock", probeClockLocal, "tools.ClockFromContext(ctx)"},
		// Python's handler takes the arguments and the configuration and
		// nothing else, so it holds a cancellation under no name at all.
		{probeLanguagePython, "cancellation", "", ""},
		{probeLanguagePython, "config", probeConfigLocal, probeConfigLocal},
		{probeLanguagePython, "clock", probeClockLocal, "now()"},
	}

	for _, testCase := range cases {
		t.Run(testCase.language+"/"+testCase.reading, func(t *testing.T) {
			t.Parallel()

			reading := testCase.reading

			local, handed, rendered := toolgen.ProbeAmbientRendered(testCase.language, reading)
			if !rendered {
				t.Fatalf("%s states no rendering for %s", testCase.language, testCase.reading)
			}

			if local != testCase.local {
				t.Errorf("local = %q, want %q", local, testCase.local)
			}

			if handed != testCase.handed {
				t.Errorf("handed = %q, want %q", handed, testCase.handed)
			}
		})
	}
}

// probeClockLocal is the local both engines take the reference instant under.
const probeClockLocal = "clock"

// TestRefusesAReadingARegisteredLanguageStatesNoExpressionFor covers the other
// half of the gap a tenth language opens: it types a reading and states nothing
// its handler can fill the parameter from.
//
// Go answers that as a build failure in emitted code. Python answers it only on
// the first call that reaches the operation, which is the dynamic-language hole
// the generated surface exists to close, so the emitter refuses it in both.
func TestRefusesAReadingARegisteredLanguageStatesNoExpressionFor(t *testing.T) {
	t.Parallel()

	for _, language := range []string{probeLanguageGo, probeLanguagePython} {
		t.Run(language, func(t *testing.T) {
			t.Parallel()

			err := toolgen.ProbeAmbientUnhanded(language, "clock")

			if want := refusalNamed(t, "errLocalArmAmbientUnhanded"); !errors.Is(err, want) {
				t.Errorf("refusal = %v, want errLocalArmAmbientUnhanded (%v)", err, want)
			}
		})
	}
}

// TestEveryRenderedReadingStatesItsExpression is the case that says the one
// above measures something: no shipped row types a reading and leaves the
// handler nothing to fill it from.
func TestEveryRenderedReadingStatesItsExpression(t *testing.T) {
	t.Parallel()

	for _, language := range []string{probeLanguageGo, probeLanguagePython} {
		for _, reading := range toolgen.ProbeAmbientVocabulary() {
			local, handed, rendered := toolgen.ProbeAmbientRendered(language, reading)
			if rendered && local != "" && handed == "" {
				t.Errorf("%s types %s as %q and hands over nothing", language, reading, local)
			}
		}
	}
}

// TestEveryDeclaredOperationWritesItsArm is the case that says the perturbing
// ones measure something: every operation the contract declares writes an arm
// without refusing, the two-sided one on both sides. It is also what says the
// arm layer is retired, because nothing exempts an operation from it any more.
func TestEveryDeclaredOperationWritesItsArm(t *testing.T) {
	t.Parallel()

	if err := toolgen.ProbeGeneratedArms(nil); err != nil {
		t.Errorf("the shipped declarations refused: %v", err)
	}
}

// TestEveryGeneratedOperationDerivesItsSubsystemSurface pins the derivation to
// the methods go/internal/tools and python/src/linodemcp/tools actually define.
//
// Spelled out rather than computed: a derivation checked against itself would
// pass while renaming every method the emitted arm calls, and the engines are
// what the emitted call has to land on.
func TestEveryGeneratedOperationDerivesItsSubsystemSurface(t *testing.T) {
	t.Parallel()

	engines := []struct {
		member    string
		direction string
		typed     string
		goName    string
		pyName    string
	}{
		{memberBuildInfo, probeDirectionNone, "BuildInfoSubsystem", "BuildInfo", "build_info"},
		{
			memberDraftCreate, probeDirectionNone, "DraftCreateSubsystem",
			"DraftCreate", "draft_create",
		},
		{memberDraftRead, probeDirectionNone, "DraftReadSubsystem", "DraftRead", "draft_read"},
		{
			memberDraftDiscard, probeDirectionNone, "DraftDiscardSubsystem",
			"DraftDiscard", "draft_discard",
		},
		{memberDraftSet, probeDirectionNone, "DraftSetSubsystem", "DraftSet", "draft_set"},
		{
			memberDraftSave, probeDirectionNone, "DraftSaveSubsystem",
			"DraftSave", "draft_save",
		},
		{
			probeDraftToolsMember, probeDirectionAdd, "DraftToolsSubsystem",
			"DraftToolsAdd", "draft_tools_add",
		},
		{
			probeDraftToolsMember, probeDirectionRemove, "DraftToolsSubsystem",
			"DraftToolsRemove", "draft_tools_remove",
		},
		{
			memberCatalogTools, probeDirectionNone, "CatalogToolsSubsystem",
			"CatalogTools", "catalog_tools",
		},
		{
			memberCatalogCategories, probeDirectionNone, "CatalogCategoriesSubsystem",
			"CatalogCategories", "catalog_categories",
		},
	}

	// A member no name derives from spells nothing rather than guessing, which
	// is the answer the run refuses on ahead of any rendering.
	if got := toolgen.ProbeSubsystemType("DRAFT_READ"); got != "" {
		t.Errorf("undeclared member type = %q, want empty", got)
	}

	for _, engine := range engines {
		t.Run(engine.member+engine.direction, func(t *testing.T) {
			t.Parallel()

			if got := toolgen.ProbeSubsystemType(engine.member); got != engine.typed {
				t.Errorf("subsystem type = %q, want %q", got, engine.typed)
			}

			if got := toolgen.ProbeSubsystemMethod(
				probeLanguageGo, engine.member, engine.direction,
			); got != engine.goName {
				t.Errorf("go method = %q, want %q", got, engine.goName)
			}

			if got := toolgen.ProbeSubsystemMethod(
				probeLanguagePython, engine.member, engine.direction,
			); got != engine.pyName {
				t.Errorf("python method = %q, want %q", got, engine.pyName)
			}
		})
	}
}

// TestEveryReportedConditionDerivesItsOwnFailureName pins the failure
// vocabulary to the names each language's engine raises and catches.
func TestEveryReportedConditionDerivesItsOwnFailureName(t *testing.T) {
	t.Parallel()

	conditions := []struct {
		member string
		goName string
		pyName string
		cause  string
	}{
		{
			"LOCAL_GUARD_DRAFT_MISSING", "ErrLocalDraftMissing",
			"LocalDraftMissingError", "draft missing",
		},
		{
			"LOCAL_GUARD_READ_FAILED", "ErrLocalReadFailed",
			"LocalReadFailedError", "read failed",
		},
	}

	for _, condition := range conditions {
		t.Run(condition.member, func(t *testing.T) {
			t.Parallel()

			got := toolgen.ProbeGuardSentinel(probeLanguageGo, condition.member)
			if got != condition.goName {
				t.Errorf("go sentinel = %q, want %q", got, condition.goName)
			}

			if got := toolgen.ProbeGuardSentinel(probeLanguagePython, condition.member); got != condition.pyName {
				t.Errorf("python sentinel = %q, want %q", got, condition.pyName)
			}

			if got := toolgen.ProbeGuardCause(condition.member); got != condition.cause {
				t.Errorf("cause = %q, want %q", got, condition.cause)
			}
		})
	}
}
