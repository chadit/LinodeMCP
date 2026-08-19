package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
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
const (
	probePreviewPlain    = "A probe is created."
	probePreviewNamed    = "A probe {note} is created."
	probePreviewUnclosed = "A probe {note is created."
	probePreviewAbsent   = "A probe {nobody} is created."
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
			name:    "declaration beside a preview hook",
			refusal: "errPreviewSentenceWithHook",
			build: previewProbe("ProbePreviewWithHookInput", withHooks("preview"),
				&linodev1.PreviewSentence{Template: []string{probePreviewPlain}, Line: previewEffect}),
		},
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
			name:    "wording an earlier one always wins over",
			refusal: "errPreviewSentenceUnreachable",
			build: previewProbe("ProbePreviewUnreachableInput", nil,
				&linodev1.PreviewSentence{
					Template: []string{probePreviewPlain, probePreviewNamed},
					Line:     previewWarning,
				}),
		},
		{
			name:    "no-echo flag beside a preview hook",
			refusal: "errPreviewOmitsBodyWithHook",
			build: func(t *testing.T) *toolgen.ProbeRun {
				t.Helper()

				return goProbe(probeMessage(t, "ProbePreviewOmitsBodyHookInput",
					writeOptions(withHooks("preview"), withPreviewOmitsBody),
					bodyString(probeDomainArg, 1)))
			},
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
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){withPreview(sentences...)}
		if extra != nil {
			sets = append(sets, extra)
		}

		return goProbe(probeMessage(t, message, writeOptions(sets...),
			bodyString(probeDomainArg, 1)))
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
