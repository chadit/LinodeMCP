package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The preview_sentence refusals. A dry run is what a caller decides on, so a
// declaration that does not land is worse than silence: the preview reads as a
// complete account of the call while leaving out the consequence, or names a
// value nothing filled in.

// The wordings the cases declare, spelled once so a case reads as the shape it
// is about rather than as prose.
// The state member and the value a matched line's arms name, spelled once so a
// case reads as the shape it is about.
const (
	argStatusMember = "status"
	probeMatchValue = "running"
)

const (
	probePreviewPlain    = "A probe is created."
	probePreviewNamed    = "A probe {note} is created."
	probePreviewUnclosed = "A probe {note is created."
	probePreviewAbsent   = "A probe {nobody} is created."
	probePreviewElement  = "A probe {element} is created."
)

// The line a case declares its wording on, spelled once because every case but
// the unspecified one wants the same half.
const (
	previewEffect  = linodev1.PreviewLine_PREVIEW_LINE_SIDE_EFFECT
	previewWarning = linodev1.PreviewLine_PREVIEW_LINE_WARNING
)

func TestRefusesAPreviewDeclarationThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "line with no wording",
			refusal: "errPreviewSentenceNoTemplate",
			build: previewProbe("ProbePreviewNoTemplateInput", nil,
				&linodev1.PreviewSentence{Line: previewEffect}),
		},
		{
			name:    "wording on neither half of the preview",
			refusal: "errPreviewSentenceNoLine",
			build: previewProbe("ProbePreviewNoLineInput", nil,
				&linodev1.PreviewSentence{Template: []string{probePreviewPlain}}),
		},
		{
			name:    "placeholder whose brace never closes",
			refusal: "errPreviewSentenceUnclosed",
			build: previewProbe("ProbePreviewUnclosedInput", nil,
				&linodev1.PreviewSentence{Template: []string{probePreviewUnclosed}, Line: previewEffect}),
		},
		{
			name:    "placeholder naming an argument the message does not declare",
			refusal: "errPreviewSentenceUnknownArgument",
			build: previewProbe("ProbePreviewUnknownArgumentInput", nil,
				&linodev1.PreviewSentence{Template: []string{probePreviewAbsent}, Line: previewEffect}),
		},
		{
			name:    "placeholder on an argument named as a secret",
			refusal: "errPreviewSentenceSecretArgument",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewSecretInput",
					writeOptions(withPreview(&linodev1.PreviewSentence{
						Template: []string{"A probe {" + probeSecretArg + "} is created."},
						Line:     previewEffect,
					})),
					bodyString(probeDomainArg, 1), bodyString(probeSecretArg, 2)))
			},
		},
		{
			name:    "placeholder on an argument a preview stands in for",
			refusal: "errPreviewSentenceSecretArgument",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewRedactedInput",
					writeOptions(withPreview(&linodev1.PreviewSentence{
						Template: []string{probePreviewNamed}, Line: previewEffect,
					})),
					bodyString(probeDomainArg, 1, withRedact)))
			},
		},
		{
			name:    "placeholder on an argument with no one spelling",
			refusal: "errPreviewSentenceUnreportable",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewUnreportableInput",
					writeOptions(withPreview(&linodev1.PreviewSentence{
						Template: []string{"A probe {" + probeBodyArg + "} is created."},
						Line:     previewEffect,
					})),
					bodyString(probeDomainArg, 1), bodyBool(probeBodyArg, 2)))
			},
		},
		{
			name:    "wording reading state on a tool that reads none",
			refusal: "errPreviewSentenceNoState",
			build: previewProbe("ProbePreviewNoStateInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{"A probe {state:label} is created."}, Line: previewEffect,
				}),
		},
		{
			name:    "wording reading a member the read does not answer with",
			refusal: "errPreviewStateUnknownMember",
			build: previewStateBesideProbe("ProbePreviewStateMemberInput",
				"A probe {state:nobody} is created.", nil, nil),
		},
		{
			name:    "wording reading past the one member level the contract models",
			refusal: "errPreviewStateDeepMember",
			build: previewStateBesideProbe("ProbePreviewStateDeepInput",
				"A probe {state:"+probeDomainArg+".} is created.", nil, nil),
		},
	})
}

// The refusals over the CONDITIONS a line is reported under: the arguments the
// call has to carry, the change it has to be making, and the value that picks
// its wording. Split from the wording refusals above because the two ask
// different questions and one table of both had outgrown reading.
func TestRefusesAPreviewConditionThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "guard beside a flag that already selects the wording",
			refusal: "errPreviewPresentWithChoice",
			build: previewProbe("ProbePreviewGuardWithChoiceInput", nil,
				&linodev1.PreviewSentence{
					Line:        previewEffect,
					WhenPresent: []string{probeDomainArg},
					ChosenBy: &linodev1.PreviewChoice{
						Argument: probeBodyArg, WhenTrue: probePreviewPlain,
					},
				}),
		},
		{
			name:    "guard on an argument the message does not declare",
			refusal: "errPreviewSentenceUnknownArgument",
			build: previewProbe("ProbePreviewGuardUnknownInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain}, Line: previewEffect,
					WhenPresent: []string{probeAbsentName},
				}),
		},
		{
			name:    "unchanged pair holding an argument the message does not declare",
			refusal: "errPreviewUnchangedUnknownArgument",
			build: previewProbe("ProbePreviewUnchangedUnknownInput",
				withUnchanged(&linodev1.PreviewUnchanged{State: keyLabelName, Argument: probeAbsentName}),
				&linodev1.PreviewSentence{Template: []string{probePreviewPlain}, Line: previewEffect}),
		},
		{
			name:    "unchanged pair no wording reads",
			refusal: "errPreviewUnchangedUnread",
			build: previewProbe("ProbePreviewUnchangedUnreadInput",
				withUnchanged(&linodev1.PreviewUnchanged{State: keyLabelName, Argument: probeDomainArg}),
				&linodev1.PreviewSentence{Template: []string{probePreviewPlain}, Line: previewEffect}),
		},
	})
}

// The refusals over the LIST a line is written across: whether it names one,
// whether the entries can be spelled, and whether a per-entry line can share
// its half. Split from the conditions above because a list changes how many
// lines a half has rather than whether one is reported.
// The refusals over what a matched line SELECTS on. Split from the conditions
// above because a selector decides which wording a line takes rather than
// whether the line is reported at all.
func TestRefusesAMatchedLineThatCannotSelect(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "matched line selecting on both an argument and a state",
			refusal: "errPreviewMatchSelector",
			build: previewProbe("ProbePreviewMatchBothSelectorsInput", nil,
				&linodev1.PreviewSentence{
					MatchedBy: &linodev1.PreviewMatch{
						Argument: probeDomainArg,
						State:    argStatusMember,
						Arm: []*linodev1.PreviewMatchArm{
							{Equals: probeMatchValue, Wording: probePreviewPlain},
						},
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "matched line selecting on nothing",
			refusal: "errPreviewMatchSelector",
			build: previewProbe("ProbePreviewMatchNoSelectorInput", nil,
				&linodev1.PreviewSentence{
					MatchedBy: &linodev1.PreviewMatch{
						Arm: []*linodev1.PreviewMatchArm{
							{Equals: probeMatchValue, Wording: probePreviewPlain},
						},
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "matched line selecting on a state the tool never reads",
			refusal: "errPreviewSentenceNoState",
			build: previewProbe("ProbePreviewMatchStateNoReadInput", nil,
				&linodev1.PreviewSentence{
					MatchedBy: &linodev1.PreviewMatch{
						State: argStatusMember,
						Arm: []*linodev1.PreviewMatchArm{
							{Equals: probeMatchValue, Wording: probePreviewPlain},
						},
					},
					Line: previewEffect,
				}),
		},
	})
}

func TestRefusesAPreviewListThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "entry read on a line written once",
			refusal: "errPreviewElementWithoutList",
			build: previewProbe("ProbePreviewElementNoListInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewElement}, Line: previewEffect,
				}),
		},
		{
			name:    "list a line is written over and never reports",
			refusal: "errPreviewElementUnread",
			build: previewProbe("ProbePreviewElementUnreadInput", nil,
				&linodev1.PreviewSentence{
					Template:   []string{probePreviewPlain},
					Line:       previewEffect,
					PerElement: &linodev1.PreviewElements{Argument: probeListArg},
				}),
		},
		{
			name:    "list that is not a repeated argument",
			refusal: "errPreviewElementNotRepeated",
			build: previewProbe("ProbePreviewElementNotListInput", nil,
				&linodev1.PreviewSentence{
					Template:   []string{probePreviewElement},
					Line:       previewEffect,
					PerElement: &linodev1.PreviewElements{Argument: probeDomainArg},
				}),
		},
		{
			name:    "entries with no one spelling both languages write",
			refusal: "errPreviewElementUnreportable",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				flags := bodyBool(probeBodyArg, probeExtraBodyNumber)
				flags.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

				return goProbe(probeMessage(t, "ProbePreviewElementUnreportableInput",
					writeOptions(withPreview(&linodev1.PreviewSentence{
						Template:   []string{probePreviewElement},
						Line:       previewEffect,
						PerElement: &linodev1.PreviewElements{Argument: probeBodyArg},
					})),
					bodyString(probeDomainArg, 1), flags))
			},
		},
		{
			name:    "per-entry line beside another in the same half",
			refusal: "errPreviewElementBesideLine",
			build: previewListProbe("ProbePreviewElementBesideInput",
				&linodev1.PreviewSentence{
					Template:   []string{probePreviewElement},
					Line:       previewEffect,
					PerElement: &linodev1.PreviewElements{Argument: probeListArg},
				},
				&linodev1.PreviewSentence{Template: []string{probePreviewPlain}, Line: previewEffect}),
		},
		{
			name:    "number guarded on one line and read unguarded on another",
			refusal: "errPreviewNumberGuardMixed",
			build: previewNumberProbe("ProbePreviewNumberGuardMixedInput",
				&linodev1.PreviewSentence{
					Template:    []string{"A probe holds {" + probeBodyArg + "}."},
					Line:        previewEffect,
					WhenPresent: []string{probeBodyArg},
				},
				&linodev1.PreviewSentence{
					Template: []string{"A probe still holds {" + probeBodyArg + "}."},
					Line:     previewWarning,
				}),
		},
		{
			name:    "match beside a flag that already selects the wording",
			refusal: "errPreviewMatchWithChoice",
			build: previewProbe("ProbePreviewMatchWithChoiceInput", nil,
				&linodev1.PreviewSentence{
					Line:      previewEffect,
					MatchedBy: probeMatch(),
					ChosenBy:  &linodev1.PreviewChoice{Argument: probeBodyArg, WhenTrue: probePreviewPlain},
				}),
		},
		{
			name:    "match beside a wording of its own",
			refusal: "errPreviewMatchWithTemplate",
			build: previewProbe("ProbePreviewMatchWithTemplateInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain}, Line: previewEffect, MatchedBy: probeMatch(),
				}),
		},
		{
			name:    "match on a value the message does not declare",
			refusal: "errPreviewMatchArgument",
			build: previewProbe("ProbePreviewMatchArgumentInput", nil,
				&linodev1.PreviewSentence{
					Line: previewEffect,
					MatchedBy: &linodev1.PreviewMatch{
						Argument: probeAbsentName,
						Arm:      []*linodev1.PreviewMatchArm{{Equals: compositeMemberOne, Wording: probePreviewPlain}},
					},
				}),
		},
		{
			name:    "match with nothing to select between",
			refusal: "errPreviewMatchNoArm",
			build: previewProbe("ProbePreviewMatchNoArmInput", nil,
				&linodev1.PreviewSentence{
					Line:      previewEffect,
					MatchedBy: &linodev1.PreviewMatch{Argument: probeDomainArg, Otherwise: probePreviewPlain},
				}),
		},
		{
			name:    "arm carrying no wording",
			refusal: "errPreviewMatchArmEmpty",
			build: previewProbe("ProbePreviewMatchArmEmptyInput", nil,
				&linodev1.PreviewSentence{
					Line: previewEffect,
					MatchedBy: &linodev1.PreviewMatch{
						Argument: probeDomainArg,
						Arm:      []*linodev1.PreviewMatchArm{{Equals: compositeMemberOne}},
					},
				}),
		},
		{
			name:    "one matched value named twice",
			refusal: "errPreviewMatchArmRepeated",
			build: previewProbe("ProbePreviewMatchRepeatedInput", nil,
				&linodev1.PreviewSentence{
					Line: previewEffect,
					MatchedBy: &linodev1.PreviewMatch{
						Argument: probeDomainArg,
						Arm: []*linodev1.PreviewMatchArm{
							{Equals: compositeMemberOne, Wording: probePreviewPlain},
							{Equals: compositeMemberOne, Wording: probePreviewNamed},
						},
					},
				}),
		},
		{
			name:    "change pair holding an argument the message does not declare",
			refusal: "errPreviewChangedUnknownArgument",
			build: previewProbe("ProbePreviewChangedUnknownInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain}, Line: previewEffect,
					WhenChanged: &linodev1.PreviewUnchanged{State: keyLabelName, Argument: probeAbsentName},
				}),
		},
		{
			name:    "one reading held both ways",
			refusal: "errPreviewChangedWithUnchanged",
			build: previewStateBesideProbe("ProbePreviewChangedBothWaysInput",
				"A probe {state:"+probeDomainArg+"} is created.",
				withUnchanged(&linodev1.PreviewUnchanged{State: probeDomainArg, Argument: probeDeleteArg}),
				&linodev1.PreviewUnchanged{State: probeDomainArg, Argument: probeDeleteArg}),
		},
		{
			name:    "wording an earlier one always wins over",
			refusal: "errPreviewSentenceUnreachable",
			build: previewProbe("ProbePreviewUnreachableInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain, probePreviewNamed},
					Line:     previewWarning,
				}),
		},
		{
			name:    "chosen line also carrying ordered wordings",
			refusal: "errPreviewChoiceWithTemplate",
			build: previewProbe("ProbePreviewChoiceTemplateInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain},
					ChosenBy: &linodev1.PreviewChoice{
						Argument: probeBodyArg, WhenTrue: probePreviewPlain,
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "choice selecting on nothing",
			refusal: "errPreviewChoiceNoArgument",
			build: previewProbe("ProbePreviewChoiceNoArgumentInput", nil,
				&linodev1.PreviewSentence{
					ChosenBy: &linodev1.PreviewChoice{WhenTrue: probePreviewPlain},
					Line:     previewEffect,
				}),
		},
		{
			name:    "choice whose every arm is empty",
			refusal: "errPreviewChoiceNoWording",
			build: previewProbe("ProbePreviewChoiceNoWordingInput", nil,
				&linodev1.PreviewSentence{
					ChosenBy: &linodev1.PreviewChoice{Argument: probeBodyArg},
					Line:     previewEffect,
				}),
		},
		{
			name:    "choice selecting on an argument the message does not declare",
			refusal: "errPreviewChoiceUnknownArgument",
			build: previewProbe("ProbePreviewChoiceUnknownInput", nil,
				&linodev1.PreviewSentence{
					ChosenBy: &linodev1.PreviewChoice{
						Argument: "nobody", WhenTrue: probePreviewPlain,
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "choice selecting on a value that is not a flag",
			refusal: "errPreviewChoiceNotFlag",
			build: previewProbe("ProbePreviewChoiceNotFlagInput", nil,
				&linodev1.PreviewSentence{
					ChosenBy: &linodev1.PreviewChoice{
						Argument: probeDomainArg, WhenTrue: probePreviewPlain,
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "choice reading a member of an argument carrying none",
			refusal: "errPreviewChoiceNotObject",
			build: previewProbe("ProbePreviewChoiceNotObjectInput", nil,
				&linodev1.PreviewSentence{
					ChosenBy: &linodev1.PreviewChoice{
						Argument: probeDomainArg + ".enabled", WhenTrue: probePreviewPlain,
					},
					Line: previewEffect,
				}),
		},
		{
			name:    "choice reading a member more than one level down",
			refusal: "errPreviewChoiceDeepMember",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewChoiceDeepInput",
					writeOptions(withPreview(&linodev1.PreviewSentence{
						ChosenBy: &linodev1.PreviewChoice{
							Argument: probeObjectArg + ".addresses.enabled",
							WhenTrue: probePreviewPlain,
						},
						Line: previewEffect,
					})),
					bodyString(probeDomainArg, 1), objectField(probeExtraBodyNumber)))
			},
		},
		{
			name:    "no-echo flag on a tool that sends no body",
			refusal: "errPreviewOmitsBodyWithoutBody",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewOmitsNoBodyInput",
					getOptions(withPreviewOmitsBody), pathInt(probeIDArg)))
			},
		},
	})
}

// previewProbe is the base mutation carrying whatever prose the case declares.
func previewProbe(
	message string,
	extra func(*descriptorpb.MessageOptions),
	sentences ...*linodev1.PreviewSentence,
) func(t *testing.T) *toolgen.ProbeRun {
	return previewStateProbe(message, extra, false, sentences...)
}

// previewNumberProbe is a preview over a tool carrying one whole number, for the
// cases about how a guard changes the way that number is read.
func previewNumberProbe(
	message string, sentences ...*linodev1.PreviewSentence,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(withPreview(sentences...)),
			bodyString(probeDomainArg, 1), bodyInt()))
	}
}

// bodyStringList is one repeated text argument the call sends in its body.
func bodyStringList(name string, number int32) *descriptorpb.FieldDescriptorProto {
	field := bodyString(name, number)
	field.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

	return field
}

// previewListProbe is a preview over a tool carrying one repeated argument, for
// the cases about how a per-entry line sits beside its neighbors.
func previewListProbe(
	message string, sentences ...*linodev1.PreviewSentence,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(withPreview(sentences...)),
			bodyString(probeDomainArg, 1), bodyStringList(probeListArg, probeExtraBodyNumber)))
	}
}

// probeMatch is a match that resolves on its own, for the cases about what a
// match may not sit beside.
func probeMatch() *linodev1.PreviewMatch {
	return &linodev1.PreviewMatch{
		Argument: probeDomainArg,
		Arm:      []*linodev1.PreviewMatchArm{{Equals: compositeMemberOne, Wording: probePreviewPlain}},
	}
}

// keyLabelName is the state member the unchanged-pair cases name, which no
// probe read answers with, so the pair is held on its own terms.
const keyLabelName = "label"

// withUnchanged declares the readings a wording reports only where they differ
// from the argument the call would set.
func withUnchanged(pairs ...*linodev1.PreviewUnchanged) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_PreviewUnchanged, pairs)
	}
}

// bothOptions applies two option writers where a case needs a pair of them.
func bothOptions(first, second func(*descriptorpb.MessageOptions)) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		first(options)
		second(options)
	}
}

// previewStateBesideProbe is a preview declaring its own read with that read
// declared beside it, so the refusal comes from what the wording names rather
// than from the read's absence.
func previewStateBesideProbe(
	message, template string,
	extra func(*descriptorpb.MessageOptions),
	changed *linodev1.PreviewUnchanged,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sibling := probeMessage(t, message+"Read",
			messageOptions(
				withRoute("GET", probeStateReadPath),
				withCapability(linodev1.ToolCapability_TOOL_CAPABILITY_READ),
				withDescription("Reads a probe."),
				withResponse("linode.mcp.v1.MessageResponse"),
			),
			bodyString(probeDomainArg, 1))

		sets := withStateRoute(&linodev1.StateRoute{Tool: probeStateReadTool})
		if extra != nil {
			sets = bothOptions(sets, extra)
		}

		run := previewStateProbe(message, sets, true,
			&linodev1.PreviewSentence{
				Template: []string{template}, Line: previewEffect, WhenChanged: changed,
			})(t)
		run.Beside = map[string]protoreflect.MessageDescriptor{probeStateReadTool: sibling}

		return run
	}
}

// previewStateProbe is previewProbe for the cases whose declared read is the
// preview's own, which is what a dry run makes it.
func previewStateProbe(
	message string,
	extra func(*descriptorpb.MessageOptions),
	previews bool,
	sentences ...*linodev1.PreviewSentence,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){withPreview(sentences...)}
		if extra != nil {
			sets = append(sets, extra)
		}

		fields := []*descriptorpb.FieldDescriptorProto{
			bodyString(probeDomainArg, 1), bodyStringList(probeListArg, probeExtraBodyNumber),
		}
		if previews {
			fields = []*descriptorpb.FieldDescriptorProto{
				pathInt(probeDeleteArg),
				probeField("dry_run", 4, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
					fieldOptions(withLocation(localLocation))),
			}
		}

		return goProbe(probeMessage(t, message, writeOptions(sets...), fields...))
	}
}

func withPreview(sentences ...*linodev1.PreviewSentence) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_PreviewSentence, sentences)
	}
}

func withPreviewOmitsBody(options *descriptorpb.MessageOptions) {
	proto.SetExtension(options, linodev1.E_PreviewOmitsBody, true)
}

// bodyBool is one boolean member of the request body, which is a value neither
// language spells the same way in prose.
func bodyBool(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
		fieldOptions(withLocation(bodyLocation)))
}

// The preview_stand_in refusals. A stand-in that does not land leaves the value
// it was declared over in the report, which is the one outcome a declaration
// about security material must never have.

// The repeated argument the stand-in cases are declared over, and the text they
// report in its member's place.
const (
	probeStandInList = "answers"
	probeStandInText = "[redacted]"
)

func TestRefusesAStandInThatCannotLand(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "stand-in over an argument that carries no entries",
			refusal: "errPreviewStandInNotItemList",
			build: standInProbe("ProbeStandInNotAListInput",
				standIn(probeDomainArg, probeItemMember, probeStandInText)),
		},
		{
			name:    "stand-in over a member the entries do not declare",
			refusal: "errPreviewStandInUnknownMember",
			build: standInProbe("ProbeStandInUnknownMemberInput",
				standIn(probeStandInList, "nobody", probeStandInText)),
		},
		{
			name:    "stand-in with nothing to report",
			refusal: "errPreviewStandInNoText",
			build: standInProbe("ProbeStandInNoTextInput",
				standIn(probeStandInList, probeItemMember, "")),
		},
		{
			name:    "one member stood in for twice",
			refusal: "errPreviewStandInRepeated",
			build: standInProbe("ProbeStandInRepeatedInput",
				standIn(probeStandInList, probeItemMember, probeStandInText),
				standIn(probeStandInList, probeItemMember, probeStandInText)),
		},
		{
			name:    "stand-in on a tier whose driver carries none",
			refusal: "errPreviewStandInTier",
			build: standInProbe("ProbeStandInWriteTierInput",
				standIn(probeStandInList, probeItemMember, probeStandInText)),
		},
	})
}

// standInProbe is a mutation carrying one typed list, with whatever stand-ins
// the case declares over it. The base is the write tier, which is what makes
// the tier case the same probe as the rest with nothing added.
func standInProbe(
	message string,
	stood ...*linodev1.PreviewStandIn,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, message, writeOptions(withStandIn(stood...)),
			bodyString(probeDomainArg, 1), messageListField(probeStandInList, 2)))
	}
}

// standIn is one declared stand-in, spelled here so a case reads as the shape
// it is about.
func standIn(argument, member, text string) *linodev1.PreviewStandIn {
	return &linodev1.PreviewStandIn{Argument: argument, Member: member, Text: text}
}

func withStandIn(stood ...*linodev1.PreviewStandIn) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_PreviewStandIn, stood)
	}
}

// messageListField is one array of typed messages the request body carries,
// which is the shape a stand-in names a member of.
func messageListField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	entry := probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		fieldOptions(withLocation(bodyLocation)))
	entry.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	entry.TypeName = new(probeItemMarker)

	return entry
}
